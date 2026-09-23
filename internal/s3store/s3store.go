// Package s3store is the durable artifact tier: a content-addressed object
// store (S3 or any S3-compatible endpoint such as MinIO) that outlives the
// local disk cache. Build artifacts are Saved here right after they publish
// locally; when retention later evicts the local copy, the object persists, so
// a rebuild Opens it and skips the build instead of re-running every step.
//
// Objects are keyed by the same content-address the local store uses, so the
// tier is a trivial immutable key→blob map:
//
//	<prefix>/<repo>/<side>/<hash>.tar.zst     side ∈ {fe, be, dl}
//
// The codec is explicit in the suffix and chosen at read time, so the on-disk
// format stays additive. A truncated blob must never land under a
// content-addressed key (Save's skip-if-exists would make it permanent), so an
// upload is compressed to a temp file first — a walk/tar failure aborts before
// any bytes are put — and Open verifies the decompressed length against
// metadata recorded at Save time.
package s3store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// User-metadata keys carrying the uncompressed tar's byte size and file count.
// Size is verified on Open; both give a future reconcile pass an integrity
// check beyond "the object exists".
const (
	metaUncompressedSize = "uncompressed-size"
	metaFileCount        = "file-count"
)

// ErrSourceGone reports that the source directory (or a file within it)
// vanished mid-walk — retention's GC deleted the just-built artifact out from
// under a persist. Callers treat it as a benign drop, not a failure to retry.
var ErrSourceGone = errors.New("artifact source directory disappeared during upload")

// Config configures the object-store connection. Endpoint and Bucket are
// required; everything else is optional.
type Config struct {
	Endpoint string // host:port, e.g. s3.amazonaws.com or minio:9000
	Bucket   string
	Prefix   string // optional key prefix within the bucket
	Region   string
	// AccessKey and SecretKey are an explicit static keypair — MinIO, or any
	// endpoint with no ambient identity. Leave both empty to resolve
	// credentials from the environment instead; see credsFor.
	AccessKey string
	SecretKey string
	UseSSL    bool
	// TmpDir is where an upload is compressed before it's put. Empty uses the
	// OS default; set it to the store's tmp dir so staging shares the
	// artifacts filesystem.
	TmpDir string
}

// Tier is a durable content-addressed artifact store.
type Tier struct {
	client *minio.Client
	bucket string
	prefix string
	tmpDir string
}

// credsFor resolves how the tier authenticates.
//
// An explicit keypair wins, and is the only way to reach an endpoint with no
// ambient identity (MinIO in compose, a dev bucket). Otherwise the tier falls
// back to a chain, because the deployed orchestrator authenticates to S3 as its
// EC2 instance role: there is no keypair to configure, and inventing one would
// mean minting a long-lived IAM user and keeping its secret somewhere, to buy
// nothing the instance profile does not already provide.
//
// The chain also fixes what a static keypair cannot express. Role credentials
// are temporary and carry a session token, so lifting AWS_ACCESS_KEY_ID and
// AWS_SECRET_ACCESS_KEY into a static V4 signer while dropping
// AWS_SESSION_TOKEN produces a signature the service rejects. EnvAWS reads all
// three together, and IAM refreshes from IMDS before expiry rather than
// signing with a keypair that has gone stale.
func credsFor(cfg Config) *credentials.Credentials {
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		return credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	}
	return credentials.NewChainCredentials([]credentials.Provider{
		&credentials.EnvAWS{},
		&credentials.EnvMinio{},
		&credentials.IAM{},
	})
}

// New dials the endpoint and verifies the bucket exists. It fails closed: a
// misconfigured or unreachable bucket returns an error rather than a Tier that
// would silently drop every artifact.
func New(cfg Config) (*Tier, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 tier: endpoint and bucket are required")
	}
	// Half a keypair is a typo, not a request to fall back to the environment.
	// Silently ignoring it would authenticate as something the operator did not
	// ask for — or, with no ambient identity, fail with an opaque 403.
	if (cfg.AccessKey == "") != (cfg.SecretKey == "") {
		return nil, fmt.Errorf("s3 tier: access key and secret key must be set together (leave both empty to use the ambient AWS environment)")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credsFor(cfg),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 tier: %w", err)
	}
	ok, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 tier: check bucket %q: %w", cfg.Bucket, err)
	}
	if !ok {
		return nil, fmt.Errorf("s3 tier: bucket %q does not exist", cfg.Bucket)
	}
	return &Tier{client: client, bucket: cfg.Bucket, prefix: cfg.Prefix, tmpDir: cfg.TmpDir}, nil
}

// key is the object name for a content-addressed artifact side.
func (t *Tier) key(repo, side, hash string) string {
	return path.Join(t.prefix, repo, side, hash+".tar.zst")
}

// UsageBytes sums the compressed size of every object under the tier's prefix —
// the durable footprint of all persisted artifacts. Informational: callers
// (storage reporting) treat an error as "unknown" rather than fatal, and never
// let it fail the report.
func (t *Tier) UsageBytes(ctx context.Context) (int64, error) {
	var total int64
	for obj := range t.client.ListObjects(ctx, t.bucket, minio.ListObjectsOptions{
		Prefix:    t.prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return total, fmt.Errorf("s3 tier: list %q: %w", t.prefix, obj.Err)
		}
		total += obj.Size
	}
	return total, nil
}

// Save tars srcDir's contents, zstd-compresses them, and uploads the result
// under the content-addressed key. It is a no-op if the object already exists
// (content-addressing makes an identical key byte-identical). It returns
// ErrSourceGone if the source directory disappears mid-walk (a GC race).
func (t *Tier) Save(ctx context.Context, repo, side, hash, srcDir string) error {
	key := t.key(repo, side, hash)
	if _, err := t.client.StatObject(ctx, t.bucket, key, minio.StatObjectOptions{}); err == nil {
		return nil // already present
	} else if !isNotFound(err) {
		return fmt.Errorf("s3 tier: stat %s: %w", key, err)
	}

	// Two-pass: compress to a temp file so the object's size and integrity
	// metadata are known before the put, and a walk/tar failure aborts before
	// any bytes reach the bucket.
	tmp, err := os.CreateTemp(t.tmpDir, "s3-artifact-*.tar.zst")
	if err != nil {
		return fmt.Errorf("s3 tier: stage upload: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	zw, err := zstd.NewWriter(tmp)
	if err != nil {
		return fmt.Errorf("s3 tier: zstd: %w", err)
	}
	size, count, err := tarDir(zw, srcDir)
	if err != nil {
		zw.Close()
		return err // includes ErrSourceGone, propagated verbatim
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("s3 tier: finish compression: %w", err)
	}
	compressed, err := tmp.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("s3 tier: size staged upload: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("s3 tier: rewind staged upload: %w", err)
	}

	_, err = t.client.PutObject(ctx, t.bucket, key, tmp, compressed, minio.PutObjectOptions{
		ContentType: "application/zstd",
		UserMetadata: map[string]string{
			metaUncompressedSize: strconv.FormatInt(size, 10),
			metaFileCount:        strconv.FormatInt(count, 10),
		},
	})
	if err != nil {
		return fmt.Errorf("s3 tier: put %s: %w", key, err)
	}
	return nil
}

// ObjectInfo is a stored artifact object's recorded integrity metadata: the
// uncompressed tar's byte size and file count (both written at Save time) plus
// the compressed object size. A reconcile pass compares these against a
// resident local copy to verify integrity beyond mere existence.
type ObjectInfo struct {
	UncompressedSize int64
	FileCount        int64
	CompressedSize   int64
}

// Stat returns a stored artifact side's recorded metadata, or found=false when
// the object is absent. It is a HEAD — no bytes are downloaded — so the
// reconcile pass can check existence and metadata consistency cheaply.
func (t *Tier) Stat(ctx context.Context, repo, side, hash string) (ObjectInfo, bool, error) {
	key := t.key(repo, side, hash)
	info, err := t.client.StatObject(ctx, t.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, false, nil
		}
		return ObjectInfo{}, false, fmt.Errorf("s3 tier: stat %s: %w", key, err)
	}
	return ObjectInfo{
		UncompressedSize: metaInt(info.UserMetadata, metaUncompressedSize),
		FileCount:        metaInt(info.UserMetadata, metaFileCount),
		CompressedSize:   info.Size,
	}, true, nil
}

// Delete removes a stored artifact object. The reconcile pass uses it to drop
// an object whose recorded metadata is missing or inconsistent before
// re-Saving it from a resident local copy (Save is skip-if-exists, so a bad
// object must be deleted before it can be replaced). A no-op if already absent.
func (t *Tier) Delete(ctx context.Context, repo, side, hash string) error {
	key := t.key(repo, side, hash)
	if err := t.client.RemoveObject(ctx, t.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 tier: remove %s: %w", key, err)
	}
	return nil
}

// Open returns a reader over the artifact's decompressed tar bytes, or
// found=false (not an error) when the object is absent. want is the
// regular-file content byte count recorded at Save time (0 for an object
// predating the metadata): the *caller* verifies it against what extraction
// wrote, because only the extractor counts the same quantity Save's tar
// writer did — an earlier version compared the raw decompressed stream length
// (headers and padding included) against content bytes and failed every
// large-artifact hydrate. The caller is responsible for the hardened
// filesystem extraction of the tar stream.
func (t *Tier) Open(ctx context.Context, repo, side, hash string) (io.ReadCloser, int64, bool, error) {
	key := t.key(repo, side, hash)
	info, err := t.client.StatObject(ctx, t.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, 0, false, nil
		}
		return nil, 0, false, fmt.Errorf("s3 tier: stat %s: %w", key, err)
	}
	obj, err := t.client.GetObject(ctx, t.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, false, fmt.Errorf("s3 tier: get %s: %w", key, err)
	}
	zr, err := zstd.NewReader(obj)
	if err != nil {
		obj.Close()
		return nil, 0, false, fmt.Errorf("s3 tier: zstd %s: %w", key, err)
	}
	return &tierReadCloser{zr: zr, obj: obj}, metaInt(info.UserMetadata, metaUncompressedSize), true, nil
}

// tierReadCloser exposes the decompressed tar stream and closes both layers.
type tierReadCloser struct {
	zr  *zstd.Decoder
	obj *minio.Object
}

func (v *tierReadCloser) Read(p []byte) (int, error) { return v.zr.Read(p) }

func (v *tierReadCloser) Close() error {
	v.zr.Close()
	return v.obj.Close()
}

// isNotFound reports whether err is S3's "no such key" (a genuinely absent
// object) rather than a transport or auth failure.
func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}

// metaInt reads an integer user-metadata value case-insensitively (minio
// canonicalizes header keys, so the exact casing round-tripped isn't
// guaranteed). Returns 0 when absent or unparseable.
func metaInt(md map[string]string, key string) int64 {
	for k, v := range md {
		if strings.EqualFold(k, key) {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}
