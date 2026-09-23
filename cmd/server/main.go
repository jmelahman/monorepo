package server

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	// The container image (alpine) ships no zoneinfo; embed Go's so a
	// --min-warm-window naming an IANA zone resolves everywhere the binary
	// runs.
	_ "time/tzdata"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/api"
	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/clone"
	"github.com/jmelahman/local-preview/internal/cloudscale"
	"github.com/jmelahman/local-preview/internal/config"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/fleet"
	"github.com/jmelahman/local-preview/internal/githuboidc"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/proxy"
	"github.com/jmelahman/local-preview/internal/reconcile"
	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/s3store"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/watch"
	"github.com/jmelahman/local-preview/internal/workerapi"
)

// version is populated at build time via -ldflags -X (see Dockerfile /
// .goreleaser.yaml). Use `git describe --tags --always --dirty` so a single
// string carries tag, distance, short sha, and dirty marker. Falls back to
// runtime/debug VCS info for dev builds without ldflags.
var version = ""

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version string `json:"version"`
}

// Build returns the build metadata for the running binary, falling back to
// runtime/debug VCS info when ldflags weren't set.
func Build() BuildInfo {
	if version != "" {
		return BuildInfo{Version: version}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		var rev string
		var modified bool
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
		if rev != "" {
			short := rev
			if len(short) > 7 {
				short = short[:7]
			}
			if modified {
				short += "-dirty"
			}
			return BuildInfo{Version: short}
		}
	}
	return BuildInfo{Version: "dev"}
}

// serveOptions carries `preview serve`'s flag values into run.
type serveOptions struct {
	addr             string
	dataDir          string
	inMemory         bool
	previewDomain    string
	previewBaseURL   string
	buildConcurrency int
	maxWarm          int
	maxWarmPerGB     float64
	maxWarmReserveGB float64
	minWarmWindow    string
	pollInterval     time.Duration
	githubSecret     string
	githubOIDCAud    string
	githubOIDCIssuer string
	ssoClientID      string
	ssoClientSecret  string
	ssoCallbackURL   string
	ssoAllowedOrg    string
	ssoAllowedTeam   string
	ssoAllowedLogins string
	ssoAllowedEmails string
	s3Endpoint       string
	s3Bucket         string
	s3Prefix         string
	s3Region         string
	s3AccessKey      string
	s3SecretKey      string
	s3UseSSL         bool

	cacheMaxArtifactBytes int64
	maxUploadBytes        int64

	reservedUpstreams []string

	onyxAuthUpstream string
	onyxAuthCookie   string

	role             string
	workerSecret     string
	workerListen     string
	workerEndpoint   string
	workerEndpoints  string
	workerHost       string
	controlListen    string
	controlEndpoint  string
	workerAdvertise  string
	workerInstanceID string
	asgName          string
	awsRegion        string
	metricsNamespace string
}

func Root() *cobra.Command {
	var opts serveOptions

	cmd := &cobra.Command{
		Use:     "preview",
		Short:   "Local preview-deployment orchestrator",
		Version: Build().Version,
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(opts)
		},
	}
	serve.Flags().StringVar(&opts.addr, "addr", ":8080", "HTTP listen address (default: $PREVIEW_ADDR or :8080). In the container image, prefer setting $PREVIEW_ADDR: its HEALTHCHECK derives the probe port from that env, so a flag-only override leaves the probe on 8080")
	serve.Flags().StringVar(&opts.dataDir, "data-dir", "", "Override data directory (default: $PREVIEW_DATA_DIR or XDG)")
	serve.Flags().BoolVar(&opts.inMemory, "in-memory", false, "Use an ephemeral in-memory SQLite database; all data is discarded on shutdown")
	serve.Flags().StringVar(&opts.previewDomain, "preview-domain", "", "Base domain previews are served under (default: $PREVIEW_DOMAIN or preview.localhost)")
	serve.Flags().StringVar(&opts.previewBaseURL, "preview-base-url", "", "Public base URL of previews, e.g. https://preview.example.com — sets the scheme, domain, and port of generated preview URLs when the server sits behind a proxy (default: $PREVIEW_BASE_URL)")
	serve.Flags().IntVar(&opts.buildConcurrency, "build-concurrency", 2, "Number of deploys built in parallel")
	serve.Flags().IntVar(&opts.maxWarm, "max-warm", 8, "Maximum concurrently running preview processes; the least-recently-used are stopped beyond it (0 = unlimited)")
	serve.Flags().Float64Var(&opts.maxWarmPerGB, "max-warm-per-gb", 0, "Derive --max-warm from this machine's RAM instead of a fixed number: (total GiB - --max-warm-reserve-gb) × this, floored at 1. Lets one launch template drive a mixed-instances fleet where each worker sizes its warm cap to the instance it landed on (default: $PREVIEW_MAX_WARM_PER_GB; 0 disables, keeping --max-warm)")
	serve.Flags().Float64Var(&opts.maxWarmReserveGB, "max-warm-reserve-gb", 1, "GiB of RAM held back for the OS, orchestrator, and cache when --max-warm-per-gb derives the cap (default: $PREVIEW_MAX_WARM_RESERVE_GB)")
	serve.Flags().StringVar(&opts.minWarmWindow, "min-warm-window", "", "Active hours for the fleet's min-warm floor, e.g. \"Mon-Fri 08:00-18:00 America/Chicago\" (tz optional, default UTC). Outside the window the floor is 0, so idle previews drain and the worker fleet can scale to zero; demand-driven scale-out is unaffected. Empty keeps the floor always active (default: $PREVIEW_MIN_WARM_WINDOW)")
	serve.Flags().DurationVar(&opts.pollInterval, "poll-interval", watch.DefaultInterval, "How often watched repos are fetched for new commits (0 disables watching)")
	serve.Flags().StringVar(&opts.githubSecret, "github-webhook-secret", "", "Shared secret validating GitHub webhook deliveries (default: $PREVIEW_GITHUB_WEBHOOK_SECRET; empty disables the endpoint)")
	serve.Flags().StringVar(&opts.githubOIDCAud, "github-oidc-audience", "", "Expected audience of GitHub Actions OIDC tokens (default: $PREVIEW_GITHUB_OIDC_AUDIENCE); setting it requires uploads to authenticate with a valid token bound to the target repo")
	serve.Flags().StringVar(&opts.githubOIDCIssuer, "github-oidc-issuer", githuboidc.DefaultIssuer, "GitHub Actions OIDC issuer (default: $PREVIEW_GITHUB_OIDC_ISSUER or the hosted issuer; override for GitHub Enterprise Server)")
	serve.Flags().StringVar(&opts.ssoClientID, "sso-github-client-id", "", "GitHub OAuth App client ID enabling interactive login (default: $PREVIEW_SSO_GITHUB_CLIENT_ID); setting it requires a sign-in for the dashboard, API, and previews")
	serve.Flags().StringVar(&opts.ssoClientSecret, "sso-github-client-secret", "", "GitHub OAuth App client secret (default: $PREVIEW_SSO_GITHUB_CLIENT_SECRET)")
	serve.Flags().StringVar(&opts.ssoCallbackURL, "sso-callback-url", "", "Public OAuth callback URL, e.g. https://preview.example.com/api/auth/callback — must match the GitHub OAuth App exactly (default: $PREVIEW_SSO_CALLBACK_URL)")
	serve.Flags().StringVar(&opts.ssoAllowedOrg, "sso-allowed-org", "", "Allow members of this GitHub org to sign in (default: $PREVIEW_SSO_ALLOWED_ORG)")
	serve.Flags().StringVar(&opts.ssoAllowedTeam, "sso-allowed-team", "", "Narrow --sso-allowed-org to this team slug (default: $PREVIEW_SSO_ALLOWED_TEAM)")
	serve.Flags().StringVar(&opts.ssoAllowedLogins, "sso-allowed-logins", "", "Comma-separated GitHub usernames allowed to sign in (default: $PREVIEW_SSO_ALLOWED_LOGINS)")
	serve.Flags().StringVar(&opts.ssoAllowedEmails, "sso-allowed-emails", "", "Comma-separated verified emails allowed to sign in (default: $PREVIEW_SSO_ALLOWED_EMAILS)")
	serve.Flags().StringVar(&opts.s3Endpoint, "s3-endpoint", "", "S3 (or MinIO) endpoint host:port enabling the durable artifact tier — built artifacts are uploaded and hydrated instead of rebuilt after eviction (default: $PREVIEW_S3_ENDPOINT; empty disables it)")
	serve.Flags().StringVar(&opts.s3Bucket, "s3-bucket", "", "Bucket for the durable artifact tier (default: $PREVIEW_S3_BUCKET; required to enable it)")
	serve.Flags().StringVar(&opts.s3Prefix, "s3-prefix", "", "Optional key prefix within the artifact-tier bucket (default: $PREVIEW_S3_PREFIX)")
	serve.Flags().StringVar(&opts.s3Region, "s3-region", "", "Region for the artifact-tier bucket (default: $PREVIEW_S3_REGION)")
	serve.Flags().StringVar(&opts.s3AccessKey, "s3-access-key", "", "Access key for the artifact tier (default: $PREVIEW_S3_ACCESS_KEY; leave unset to use the ambient AWS environment or instance role)")
	serve.Flags().StringVar(&opts.s3SecretKey, "s3-secret-key", "", "Secret key for the artifact tier (default: $PREVIEW_S3_SECRET_KEY; must be set together with --s3-access-key)")
	serve.Flags().BoolVar(&opts.s3UseSSL, "s3-use-ssl", true, "Use TLS for the artifact-tier endpoint (set false for local MinIO over http)")
	serve.Flags().Int64Var(&opts.cacheMaxArtifactBytes, "cache-max-artifact-bytes", 0, "Soft cap on resident (local-disk) artifact bytes; the coldest artifacts are swept back to the durable tier above it. Requires the artifact tier; 0 disables cache eviction and keeps every artifact resident (default: $PREVIEW_CACHE_MAX_ARTIFACT_BYTES)")
	serve.Flags().Int64Var(&opts.maxUploadBytes, "max-upload-bytes", defaultMaxUploadBytes, "Maximum bytes a CI upload may stream: the compressed request body is rejected with 413 above it, and extraction aborts if the decompressed tar exceeds it (guards against a gzip bomb filling the disk). Raise it for larger legitimate artifacts; 0 disables both caps (default: $PREVIEW_MAX_UPLOAD_BYTES)")
	serve.Flags().StringArrayVar(&opts.reservedUpstreams, "reserved-upstream", nil, "Route a fixed host under the preview domain to an upstream, as <label>=host:port (e.g. app=127.0.0.1:3100) — served behind the SSO gate but outside the deploy machinery, for always-on companion services. Repeatable (default: $PREVIEW_RESERVED_UPSTREAMS, comma-separated)")
	serve.Flags().StringVar(&opts.onyxAuthUpstream, "onyx-auth-upstream", "", "Enable onyx single-sign-on for previews: the reserved-upstream label of the one canonical onyx host that owns the Google OAuth client (e.g. app). Previews without an onyx session bounce there to log in, and the shared-secret JWT rides back across the preview domain (default: $PREVIEW_ONYX_AUTH_UPSTREAM; empty disables it)")
	serve.Flags().StringVar(&opts.onyxAuthCookie, "onyx-auth-cookie", "", "onyx session cookie the proxy watches for when --onyx-auth-upstream is set (default: $PREVIEW_ONYX_AUTH_COOKIE, else onyx's own default \"fastapiusersauth\")")
	serve.Flags().StringVar(&opts.role, "role", "all", "Serving role: 'all' (single node — API, dashboard, proxy, and local process supervision), 'control' (route previews to a worker tier), or 'worker' (supervise processes on behalf of a control node)")
	serve.Flags().StringVar(&opts.workerSecret, "worker-secret", "", "Shared secret authenticating the internal worker API in both directions (default: $PREVIEW_WORKER_SECRET)")
	serve.Flags().StringVar(&opts.workerListen, "worker-listen", "", "Private address to expose the internal worker API on, e.g. :9100 — MUST NOT be internet/ALB-reachable; empty disables it (roles 'worker'/'all')")
	serve.Flags().StringVar(&opts.workerEndpoint, "worker-endpoint", "", "Control node only: a worker's private worker-API base URL, e.g. http://10.0.1.5:9100 (default: $PREVIEW_WORKER_ENDPOINT)")
	serve.Flags().StringVar(&opts.workerEndpoints, "worker-endpoints", "", "Control node only: comma-separated worker-API base URLs forming the fleet, e.g. http://10.0.1.5:9100,http://10.0.1.6:9100 (default: $PREVIEW_WORKER_ENDPOINTS). Combined with --worker-endpoint")
	serve.Flags().StringVar(&opts.workerHost, "worker-host", "", "Control node only: the routable host the worker's preview processes are reached on, e.g. 10.0.1.5 (default: the --worker-endpoint host)")
	serve.Flags().StringVar(&opts.controlListen, "control-listen", "", "Control node only: private address to expose the worker-registration API on, e.g. :9101 — lets workers self-register instead of being hand-listed via --worker-endpoint(s), so an autoscaled worker joins the fleet on boot. MUST NOT be internet/ALB-reachable; empty disables it (default: $PREVIEW_CONTROL_LISTEN)")
	serve.Flags().StringVar(&opts.controlEndpoint, "control-endpoint", "", "Worker node only: the control node's --control-listen base URL, e.g. http://10.0.1.1:9101 — this worker registers itself there on boot and periodically, and deregisters on shutdown (default: $PREVIEW_CONTROL_ENDPOINT; empty disables self-registration)")
	serve.Flags().StringVar(&opts.workerAdvertise, "worker-advertise", "", "Worker node only: this worker's own worker-API base URL the control node should dial back, e.g. http://10.0.1.5:9100 — required with --control-endpoint unless it can be derived from a host-qualified --worker-listen (default: $PREVIEW_WORKER_ADVERTISE)")
	serve.Flags().StringVar(&opts.workerInstanceID, "worker-instance-id", "", "Worker node only: this worker's cloud instance-id (EC2 instance-id), passed to the control node with self-registration so it can scale-in-protect this node while it serves previews (default: $PREVIEW_WORKER_INSTANCE_ID; empty for a non-cloud worker)")
	serve.Flags().StringVar(&opts.asgName, "worker-asg-name", "", "Control node only: the worker tier's EC2 Auto Scaling group name. Set it to publish fleet autoscaling metrics (UnservedDemand, FleetLoad) to CloudWatch and reconcile scale-in protection on busy workers (default: $PREVIEW_WORKER_ASG_NAME; empty disables autoscaling integration)")
	serve.Flags().StringVar(&opts.awsRegion, "aws-region", "", "Control node only: AWS region for the CloudWatch/Auto Scaling autoscaling API. Defaults to the ambient SDK region (default: $AWS_REGION)")
	serve.Flags().StringVar(&opts.metricsNamespace, "metrics-namespace", "LocalPreview", "Control node only: CloudWatch namespace for published fleet metrics")
	cmd.AddCommand(serve)

	addClientCommands(cmd)

	return cmd
}

// warmSettingKey and idleSettingKey are the settings-table rows the
// dashboard's warm policy lives under; they override the --max-warm flag and
// every manifest's idle_timeout at boot and at runtime.
const (
	warmSettingKey    = "warm.max_warm"
	minWarmSettingKey = "warm.min_warm"
	idleSettingKey    = "warm.idle_seconds"
)

func run(opts serveOptions) error {
	if opts.addr == ":8080" { // still the compiled default
		if v := os.Getenv("PREVIEW_ADDR"); v != "" {
			opts.addr = v
		}
	}
	if opts.githubSecret == "" {
		opts.githubSecret = os.Getenv("PREVIEW_GITHUB_WEBHOOK_SECRET")
	}
	if opts.githubOIDCAud == "" {
		opts.githubOIDCAud = os.Getenv("PREVIEW_GITHUB_OIDC_AUDIENCE")
	}
	if iss := os.Getenv("PREVIEW_GITHUB_OIDC_ISSUER"); iss != "" && opts.githubOIDCIssuer == githuboidc.DefaultIssuer {
		opts.githubOIDCIssuer = iss
	}
	// Configuring an audience turns on OIDC auth for the upload endpoints; a
	// nil verifier leaves them unauthenticated, as before.
	var uploadAuth api.UploadVerifier
	if opts.githubOIDCAud != "" {
		uploadAuth = githuboidc.NewVerifier(opts.githubOIDCIssuer, opts.githubOIDCAud)
		log.Printf("upload endpoints require GitHub Actions OIDC (issuer %s, audience %s)",
			opts.githubOIDCIssuer, opts.githubOIDCAud)
	}
	envDefault(&opts.ssoClientID, "PREVIEW_SSO_GITHUB_CLIENT_ID")
	envDefault(&opts.ssoClientSecret, "PREVIEW_SSO_GITHUB_CLIENT_SECRET")
	envDefault(&opts.ssoCallbackURL, "PREVIEW_SSO_CALLBACK_URL")
	envDefault(&opts.ssoAllowedOrg, "PREVIEW_SSO_ALLOWED_ORG")
	envDefault(&opts.ssoAllowedTeam, "PREVIEW_SSO_ALLOWED_TEAM")
	envDefault(&opts.ssoAllowedLogins, "PREVIEW_SSO_ALLOWED_LOGINS")
	envDefault(&opts.ssoAllowedEmails, "PREVIEW_SSO_ALLOWED_EMAILS")
	if len(opts.reservedUpstreams) == 0 {
		if v := os.Getenv("PREVIEW_RESERVED_UPSTREAMS"); v != "" {
			opts.reservedUpstreams = strings.Split(v, ",")
		}
	}
	reserved, err := parseReservedUpstreams(opts.reservedUpstreams)
	if err != nil {
		return err
	}
	envDefault(&opts.onyxAuthUpstream, "PREVIEW_ONYX_AUTH_UPSTREAM")
	envDefault(&opts.onyxAuthCookie, "PREVIEW_ONYX_AUTH_COOKIE")
	if opts.onyxAuthUpstream != "" {
		if _, ok := reserved[strings.ToLower(opts.onyxAuthUpstream)]; !ok {
			return fmt.Errorf("--onyx-auth-upstream %q must also be a --reserved-upstream label", opts.onyxAuthUpstream)
		}
	}
	envDefault(&opts.s3Endpoint, "PREVIEW_S3_ENDPOINT")
	envDefault(&opts.s3Bucket, "PREVIEW_S3_BUCKET")
	envDefault(&opts.s3Prefix, "PREVIEW_S3_PREFIX")
	envDefault(&opts.s3Region, "PREVIEW_S3_REGION")
	// Deliberately no AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY fallback here.
	// Those two are only half of an AWS identity — temporary credentials also
	// carry AWS_SESSION_TOKEN — so lifting the pair into an explicit keypair
	// would build a signature the service rejects, and would shadow the
	// instance role the deployed server actually authenticates as. Left unset,
	// the tier resolves the whole environment itself (see s3store.credsFor).
	envDefault(&opts.s3AccessKey, "PREVIEW_S3_ACCESS_KEY")
	envDefault(&opts.s3SecretKey, "PREVIEW_S3_SECRET_KEY")
	envDefaultInt64(&opts.cacheMaxArtifactBytes, "PREVIEW_CACHE_MAX_ARTIFACT_BYTES")
	envOverrideInt64(&opts.maxUploadBytes, "PREVIEW_MAX_UPLOAD_BYTES", defaultMaxUploadBytes)
	envDefault(&opts.minWarmWindow, "PREVIEW_MIN_WARM_WINDOW")
	envDefault(&opts.workerSecret, "PREVIEW_WORKER_SECRET")
	envDefault(&opts.workerEndpoint, "PREVIEW_WORKER_ENDPOINT")
	envDefault(&opts.workerEndpoints, "PREVIEW_WORKER_ENDPOINTS")
	envDefault(&opts.controlListen, "PREVIEW_CONTROL_LISTEN")
	envDefault(&opts.controlEndpoint, "PREVIEW_CONTROL_ENDPOINT")
	envDefault(&opts.workerAdvertise, "PREVIEW_WORKER_ADVERTISE")
	envDefault(&opts.workerInstanceID, "PREVIEW_WORKER_INSTANCE_ID")
	envDefault(&opts.asgName, "PREVIEW_WORKER_ASG_NAME")
	if opts.awsRegion == "" {
		opts.awsRegion = os.Getenv("AWS_REGION")
	}
	if opts.maxWarmPerGB == 0 {
		if v := os.Getenv("PREVIEW_MAX_WARM_PER_GB"); v != "" {
			if f, e := strconv.ParseFloat(v, 64); e == nil {
				opts.maxWarmPerGB = f
			}
		}
	}
	if opts.maxWarmReserveGB == 1 { // still the compiled default
		if v := os.Getenv("PREVIEW_MAX_WARM_RESERVE_GB"); v != "" {
			if f, e := strconv.ParseFloat(v, 64); e == nil {
				opts.maxWarmReserveGB = f
			}
		}
	}
	switch opts.role {
	case "all", "control", "worker":
	default:
		return fmt.Errorf("--role must be one of all|control|worker, got %q", opts.role)
	}
	if opts.role == "control" && len(workerEndpoints(opts)) > 0 && opts.workerSecret == "" {
		return fmt.Errorf("--role=control with worker endpoints requires --worker-secret")
	}
	if opts.workerListen != "" && opts.workerSecret == "" {
		return fmt.Errorf("--worker-listen requires --worker-secret (the worker API is a remote-code-execution surface)")
	}
	if opts.controlListen != "" {
		if opts.role != "control" {
			return fmt.Errorf("--control-listen is a control-node flag; got --role=%q", opts.role)
		}
		if opts.workerSecret == "" {
			return fmt.Errorf("--control-listen requires --worker-secret (worker registration steers preview traffic)")
		}
	}
	if opts.controlEndpoint != "" {
		if opts.role != "worker" && opts.role != "all" {
			return fmt.Errorf("--control-endpoint is a worker-node flag; got --role=%q", opts.role)
		}
		if opts.workerSecret == "" {
			return fmt.Errorf("--control-endpoint requires --worker-secret")
		}
	}
	// Setting the client ID turns on interactive SSO for the dashboard, API,
	// and previews; leaving it unset keeps the historical open behavior.
	var sso api.SSOProvider
	var dashboardOrigin string
	var cookiesSecure bool
	if opts.ssoClientID != "" {
		provider, origin, secure, err := buildSSO(opts)
		if err != nil {
			return err
		}
		sso, dashboardOrigin, cookiesSecure = provider, origin, secure
		log.Printf("SSO login enabled (dashboard %s); dashboard, API, and previews require a GitHub sign-in", origin)
	}
	cfg, err := config.Load(config.Options{
		DataDir:        opts.dataDir,
		PreviewDomain:  opts.previewDomain,
		PreviewBaseURL: opts.previewBaseURL,
		Addr:           opts.addr,
	})
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	workCtx, stopWork := context.WithCancel(context.Background())
	defer stopWork()

	dbPath := cfg.DBPath()
	if opts.inMemory {
		log.Printf("WARNING: --in-memory set; using ephemeral SQLite, all data is lost on shutdown")
		dbPath = ":memory:"
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	files := store.New(cfg.ArtifactsDir(), cfg.StateDir(), cfg.TmpDir())
	// Bound tar expansion for every extraction this Store performs — upload and
	// durable-tier hydrate alike — so a gzip bomb can't fill the disk.
	files.SetMaxExtractBytes(opts.maxUploadBytes)
	if err := files.SweepTmp(24 * time.Hour); err != nil {
		log.Printf("sweep tmp: %v", err)
	}
	gitMgr := gitrepo.NewManager(cfg.ReposDir())
	super := supervise.New(database, files, cfg.LogsDir())
	super.ReclaimOrphans()
	// Memory-calibrated warm cap: when --max-warm-per-gb is set, derive the boot
	// --max-warm from this machine's RAM so one launch template can serve a
	// mixed-instances fleet of differently sized workers. This only sets the
	// boot default; a dashboard-saved setting still overrides below.
	if opts.maxWarmPerGB > 0 {
		if mem, merr := readMemTotalBytes(); merr != nil {
			log.Printf("warm calibration: reading memory failed (%v); keeping --max-warm %d", merr, opts.maxWarm)
		} else {
			n := warmFromMemory(mem, opts.maxWarmPerGB, opts.maxWarmReserveGB)
			log.Printf("warm calibration: %d warm slots from %.1f GiB RAM (%.2f/GiB, %.1f GiB reserved); --max-warm %d superseded",
				n, float64(mem)/(1<<30), opts.maxWarmPerGB, opts.maxWarmReserveGB, opts.maxWarm)
			opts.maxWarm = n
		}
	}
	// The warm cap: the --max-warm flag is the boot default, overridden by the
	// dashboard-saved setting when one exists (the setting survives restarts
	// and, via the fleet's reconcile loop, worker reboots).
	effectiveMaxWarm := opts.maxWarm
	warmOverridden := false
	if v, err := database.GetSetting(warmSettingKey); err == nil {
		if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
			effectiveMaxWarm, warmOverridden = n, true
		}
	}
	super.SetMaxWarm(effectiveMaxWarm)
	if warmOverridden {
		log.Printf("warm cap: %d (dashboard setting; --max-warm %d overridden)", effectiveMaxWarm, opts.maxWarm)
	}
	minWarm, minWarmSet := 0, false
	if v, err := database.GetSetting(minWarmSettingKey); err == nil {
		if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
			minWarm, minWarmSet = n, true
			super.SetMinWarm(n)
		}
	}
	idleOverrideSec, idleOverridden := 0, false
	if v, err := database.GetSetting(idleSettingKey); err == nil {
		if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
			idleOverrideSec, idleOverridden = n, true
			super.SetIdleOverride(time.Duration(n) * time.Second)
			if n > 0 {
				log.Printf("idle timeout: %ds (dashboard setting; manifest idle_timeout overridden)", n)
			}
		}
	}
	super.StartReaper(workCtx)
	queue := build.NewQueue(database, gitMgr, files, super, cfg.LogsDir(), nil)
	queue.SetManifestRefs([]build.ManifestRef{
		{Path: build.ManifestName},
		{Path: ".kanban.toml", Table: "previews"},
	})
	if dir := config.ManifestsDir(); dir != "" {
		queue.SetLocalManifestDir(dir)
	}
	// A configured bucket enables the durable artifact tier: built artifacts are
	// uploaded and, after eviction, hydrated instead of rebuilt. Fail closed so a
	// misconfigured bucket surfaces at startup rather than silently dropping every
	// upload. Must be set before Start, which launches the persist workers.
	var artifactTier *s3store.Tier
	if opts.s3Bucket != "" && opts.s3Endpoint != "" {
		t, err := s3store.New(s3store.Config{
			Endpoint:  opts.s3Endpoint,
			Bucket:    opts.s3Bucket,
			Prefix:    opts.s3Prefix,
			Region:    opts.s3Region,
			AccessKey: opts.s3AccessKey,
			SecretKey: opts.s3SecretKey,
			UseSSL:    opts.s3UseSSL,
			TmpDir:    cfg.TmpDir(),
		})
		if err != nil {
			return err
		}
		artifactTier = t
		queue.SetArtifactTier(t)
		log.Printf("artifact tier: s3 %s/%s (built artifacts persist and hydrate instead of rebuilding)",
			opts.s3Endpoint, opts.s3Bucket)
	}
	queue.Start(workCtx, opts.buildConcurrency)
	watcher := watch.New(database, gitMgr, queue, opts.pollInterval)
	watcher.Start(workCtx)
	cloner := clone.New(database, gitMgr, watcher.Kick)
	cloner.Start(workCtx)
	sweeper := retain.New(database, super, files)
	sweeper.Start(workCtx)
	// A worker's disk is a cache and nothing but — left uncapped it fills with
	// one hydrated artifact per distinct preview ever served, none of them
	// reclaimed, until hydration itself fails with ENOSPC. Default its cap
	// from the data filesystem's size; an explicit flag still wins.
	if opts.cacheMaxArtifactBytes == 0 && opts.role == "worker" && files.ArtifactTier() != nil {
		capBytes, fsBytes, err := workerCacheCap(cfg.DataDir)
		if err != nil {
			log.Printf("artifact cache: worker default cap unavailable (%v); resident artifacts are uncapped — set --cache-max-artifact-bytes", err)
		} else {
			opts.cacheMaxArtifactBytes = capBytes
			log.Printf("artifact cache: worker default cap is %.0f%% of the %d-byte filesystem under %s",
				workerCacheFraction*100, fsBytes, cfg.DataDir)
		}
	}
	// With a durable tier and a resident-cache cap set, sweep the coldest
	// artifacts off local disk periodically; a swept artifact re-hydrates on
	// next serve. Off by default (cap 0), so single-node instances keep every
	// artifact resident exactly as before.
	if opts.cacheMaxArtifactBytes > 0 && files.ArtifactTier() != nil {
		startCacheSweeper(workCtx, files, super, opts.cacheMaxArtifactBytes)
		log.Printf("artifact cache: resident cap %d bytes (coldest swept to durable tier, re-hydrated on serve)",
			opts.cacheMaxArtifactBytes)
	}
	// With a durable tier, reconcile guarantees every live deploy's artifacts
	// exist and verify in the bucket — closing gaps from pre-tier builds,
	// cache-hit builds that skipped persist, and crash-dropped uploads. It runs
	// once at startup and periodically thereafter.
	if artifactTier != nil {
		startReconciler(workCtx, reconcile.New(database, files, artifactTier))
	}

	// Serving transport: the proxy is address-based and transport-agnostic. By
	// default (and for a worker serving its own processes) it drives the local
	// Manager over loopback; a control node with a worker endpoint drives the
	// remote worker over the internal API instead. Both satisfy proxy.Backends.
	// The fleet registry is built before the API deps because it is also the
	// dashboard's runtime view (status/stats/run logs live on the workers).
	var backends proxy.Backends = supervise.LocalBackends{M: super}
	var runtime api.RuntimeView = super
	var reg *fleet.Registry
	var registrar workerRegistrar
	if opts.role == "control" {
		// Build the fleet whenever this node routes to workers at all: either a
		// static list (--worker-endpoint(s)) or a registration listener
		// (--control-listen) that workers join at runtime — including an empty
		// fleet that fills in on demand as workers scale up from zero.
		endpoints := workerEndpoints(opts)
		if len(endpoints) > 0 || opts.controlListen != "" {
			reg = fleet.New(fleetStaleAfter)
			if opts.minWarmWindow != "" {
				active, werr := fleet.ParseMinWarmWindow(opts.minWarmWindow)
				if werr != nil {
					return werr
				}
				reg.SetMinWarmWindow(active)
				log.Printf("min-warm floor active %s; outside it the floor is 0 and the fleet drains", opts.minWarmWindow)
			}
			registrar = workerRegistrar{reg: reg, super: super, secret: opts.workerSecret}
			for _, ep := range endpoints {
				host := opts.workerHost
				if host == "" || len(endpoints) > 1 {
					host = hostOf(ep)
				}
				// Hand-listed workers carry no instance-id — scale-in protection is
				// only meaningful for autoscaled, self-registering nodes.
				_ = registrar.Register(ep, host, "")
			}
			if warmOverridden {
				reg.SetMaxWarm(effectiveMaxWarm)
			}
			if minWarmSet {
				reg.SetMinWarm(minWarm)
			}
			if idleOverridden {
				reg.SetIdleOverride(time.Duration(idleOverrideSec) * time.Second)
			}
			// Worker-shipped process events land in the control's own trail:
			// they're recorded where the process runs (a worker's ephemeral
			// database), so without this the dashboard's startup percentiles
			// would read empty in fleet mode. Original timestamps preserved —
			// the percentiles are deltas between paired rows.
			reg.SetEventSink(func(evs []supervise.ProcEventRecord) {
				for _, e := range evs {
					database.AddProcessEventAt(e.RepoID, e.BeHash, e.Event, e.Detail, e.OccurredAt) //nolint:errcheck // observability trail, best-effort
				}
			})
			reg.StartHeartbeats(workCtx, fleetHeartbeatInterval)
			startFleetSignal(workCtx, reg)
			// With an ASG named, publish the scale-from-zero (UnservedDemand) and
			// load (FleetLoad) metrics to CloudWatch and reconcile scale-in
			// protection on busy workers. Best-effort: a credential/region failure
			// logs and leaves the fleet running without autoscaling integration.
			if opts.asgName != "" {
				pub, err := cloudscale.New(workCtx, reg, cloudscale.Config{
					Region:    opts.awsRegion,
					ASGName:   opts.asgName,
					Namespace: opts.metricsNamespace,
					Interval:  cloudPublishInterval,
				})
				if err != nil {
					log.Printf("cloudscale: disabled (%v)", err)
				} else {
					go pub.Run(workCtx)
					log.Printf("cloudscale: publishing fleet metrics for ASG %q (namespace %q)", opts.asgName, opts.metricsNamespace)
				}
			}
			backends = reg
			runtime = reg
			// Freshly built deploys pre-warm on the worker traffic routes to,
			// not on this node's (otherwise idle) local manager.
			queue.SetStarter(reg)
		} else {
			log.Printf("role=control with no --worker-endpoint(s) or --control-listen: previews will serve locally")
		}
	}

	deps := api.Deps{
		Store:               database,
		Build:               api.BuildInfo(Build()),
		Config:              cfg,
		Git:                 gitMgr,
		Queue:               queue,
		Super:               super,
		Runtime:             runtime,
		Cloner:              cloner,
		Watcher:             watcher,
		Files:               files,
		Sweeper:             sweeper,
		DBPath:              dbPath,
		GitHubWebhookSecret: opts.githubSecret,
		UploadAuth:          uploadAuth,
		MaxUploadBytes:      opts.maxUploadBytes,
		SSO:                 sso,
		DashboardOrigin:     dashboardOrigin,
		CookiesSecure:       cookiesSecure,
		FleetStats:          fleetStatsFn(reg),
		WarmPolicy: func() api.WarmPolicy {
			return api.WarmPolicy{
				MaxWarm:            super.MaxWarm(),
				MinWarm:            super.MinWarm(),
				IdleTimeoutSeconds: int(super.IdleOverride() / time.Second),
			}
		},
		SetWarmPolicy: func(p api.WarmPolicy) error {
			if err := database.SetSetting(warmSettingKey, strconv.Itoa(p.MaxWarm)); err != nil {
				return err
			}
			if err := database.SetSetting(minWarmSettingKey, strconv.Itoa(p.MinWarm)); err != nil {
				return err
			}
			if err := database.SetSetting(idleSettingKey, strconv.Itoa(p.IdleTimeoutSeconds)); err != nil {
				return err
			}
			idle := time.Duration(p.IdleTimeoutSeconds) * time.Second
			super.SetMaxWarm(p.MaxWarm)
			super.SetMinWarm(p.MinWarm)
			super.SetIdleOverride(idle)
			if reg != nil {
				// Workers pick these up on the next heartbeat.
				reg.SetMaxWarm(p.MaxWarm)
				reg.SetMinWarm(p.MinWarm)
				reg.SetIdleOverride(idle)
			}
			return nil
		},
	}
	apex := api.AuthMiddleware(deps, api.NewMux(deps))
	router := proxy.New(database, files, backends, cfg.Preview.Domain, apex)
	// Interim "starting" pages stream the run log from wherever the process
	// runs — the local manager, or the fleet view on a control node — and
	// consult process status so a frontend blocked on its backend's init is
	// narrated as the backend (the side actually doing the work).
	router.SetRunLogs(runtime)
	router.SetProcStatus(runtime)
	if sso != nil {
		router.SetPreviewAuth(true, dashboardOrigin, cookiesSecure)
		startSessionGC(workCtx, database)
	}
	if len(reserved) > 0 {
		router.SetReservedUpstreams(reserved)
		for label, addr := range reserved {
			log.Printf("reserved upstream: %s.%s -> %s", label, cfg.Preview.Domain, addr)
		}
	}
	if opts.onyxAuthUpstream != "" {
		router.SetOnyxAuth(opts.onyxAuthUpstream, opts.onyxAuthCookie)
		log.Printf("onyx SSO: previews log in via %s.%s (cookie %q)",
			strings.ToLower(opts.onyxAuthUpstream), cfg.Preview.Domain,
			cmp.Or(opts.onyxAuthCookie, "fastapiusersauth"))
	}

	// Expose the internal worker API when configured (worker/all roles). It is a
	// remote-code-execution surface — private listener, shared-secret only,
	// never ALB-reachable.
	var workerSrv *http.Server
	if opts.role == "worker" {
		// Container ports publish loopback-only by default, which the control
		// node's proxy can never dial. Publish them additionally on the address
		// the worker API listens on — the address control reaches this node at.
		// A host-less listen (":9100") publishes on all interfaces; either way
		// the security group / firewall owns who can actually connect.
		publishIP := "0.0.0.0"
		if host, _, err := net.SplitHostPort(opts.workerListen); err == nil && host != "" && host != "127.0.0.1" && host != "localhost" {
			publishIP = host
		}
		super.SetPublishIP(publishIP)
		log.Printf("role=worker: containered preview ports publish on %s", publishIP)
	}
	if opts.workerListen != "" && (opts.role == "worker" || opts.role == "all") {
		if isPublicBind(opts.workerListen) {
			log.Printf("WARNING: --worker-listen %q binds all interfaces — the worker API starts arbitrary processes and must be firewalled to the control node only (private subnet / security group)",
				opts.workerListen)
		}
		workerSrv = &http.Server{
			Addr:              opts.workerListen,
			Handler:           workerapi.NewServer(super, opts.workerSecret).Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			// Bound the full request-read and idle-keepalive phases so a slow-body
			// or idle client can't pin a connection open indefinitely. No
			// WriteTimeout: worker responses can stream and a blanket write
			// deadline would truncate them.
			ReadTimeout: 60 * time.Second,
			IdleTimeout: 120 * time.Second,
		}
		go func() {
			log.Printf("worker API listening on %s (private; shared-secret auth)", opts.workerListen)
			if err := workerSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("worker API listen: %v", err)
			}
		}()
	}

	// Control node: accept worker self-registration. Same RCE-adjacent trust
	// model as the worker API — private listener, shared-secret only, never
	// ALB-reachable. Only started when the fleet registry exists.
	var controlSrv *http.Server
	if opts.controlListen != "" && reg != nil {
		if isPublicBind(opts.controlListen) {
			log.Printf("WARNING: --control-listen %q binds all interfaces — worker registration steers preview traffic and must be firewalled to the worker subnet only (private subnet / security group)",
				opts.controlListen)
		}
		controlSrv = &http.Server{
			Addr:              opts.controlListen,
			Handler:           workerapi.NewControlServer(registrar, opts.workerSecret).Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		go func() {
			log.Printf("control registration API listening on %s (private; shared-secret auth)", opts.controlListen)
			if err := controlSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("control API listen: %v", err)
			}
		}()
	}

	// Worker node: self-register with the control node so an autoscaled worker
	// the control node was never configured with joins the fleet on boot.
	if opts.controlEndpoint != "" && (opts.role == "worker" || opts.role == "all") {
		advertise, err := deriveAdvertise(opts)
		if err != nil {
			return err
		}
		cc := workerapi.NewControlClient(opts.controlEndpoint, opts.workerSecret, nil)
		startWorkerRegistration(workCtx, cc, advertise, hostOf(advertise), opts.workerInstanceID)
		log.Printf("role=worker: self-registering with control at %s (advertising %s, instance %q)", opts.controlEndpoint, advertise, opts.workerInstanceID)
	}

	srv := &http.Server{
		Addr:              opts.addr,
		Handler:           recoverPanics(logRequests(compressResponses(router))),
		ReadHeaderTimeout: 10 * time.Second,
		// Bound request-read and idle-keepalive so a slow-body or idle client
		// can't hold connections open indefinitely. No WriteTimeout: the proxy
		// streams long-lived preview responses and the cold-start flow relies on
		// Retry-After, both of which a blanket write deadline would truncate;
		// per-request deadlines live in the proxy (ensureAndProxy) instead.
		ReadTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (previews at %s)", opts.addr, cfg.Preview.URL("<sha>", "<repo>"))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if workerSrv != nil {
		workerSrv.Shutdown(ctx) //nolint:errcheck
	}
	if controlSrv != nil {
		controlSrv.Shutdown(ctx) //nolint:errcheck
	}
	// Cancelling workCtx signals the self-registration loop to send a
	// best-effort deregister before the process exits.
	stopWork()
	// Drain in-flight artifact uploads before stopping processes: the build
	// workers have stopped enqueuing (their context is cancelled), so this only
	// waits on uploads already started.
	queue.Stop()
	super.StopAll()
	return err
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush, Hijack, and Unwrap forward the streaming/upgrade capabilities of the
// underlying connection — without them this logging wrapper would sever SSE
// flushes and WebSocket upgrades (the exec endpoint, or one proxied to a
// preview) from every handler downstream.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := s.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("hijack not supported")
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// recoverPanics is the outermost middleware. It catches panics from any
// downstream handler so the process survives, logs the trace, and writes a
// 500 response.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

// cacheSweepInterval is how often the resident-artifact cache is trimmed to its
// cap; cacheSweepMinAge shields an artifact published within it from eviction,
// so a just-built side whose asynchronous persist is still in flight is never
// swept before it lands durably.
const (
	// cacheSweepInterval is deliberately well above a minute: a sweep walks
	// resident artifact trees, and even with size memoization it contends with
	// previews reading the same directories, so it should be infrequent.
	cacheSweepInterval = 5 * time.Minute
	cacheSweepMinAge   = 10 * time.Minute
)

// startCacheSweeper periodically trims resident artifacts to maxBytes, coldest
// first, protecting any side with a live process. Returns immediately; the loop
// stops when ctx is cancelled.
func startCacheSweeper(ctx context.Context, files *store.Store, super *supervise.Manager, maxBytes int64) {
	go func() {
		t := time.NewTicker(cacheSweepInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// protect is evaluated per candidate immediately before deletion
				// (inside EvictCacheToWatermark), so it must be a live check — a
				// snapshot taken here would be TOCTOU against a process that
				// starts mid-sweep and bind-mounts the files about to be removed.
				protect := super.IsArtifactLive
				if freed, err := files.EvictCacheToWatermark(maxBytes, cacheSweepMinAge, protect); err != nil {
					log.Printf("artifact cache sweep: %v", err)
				} else if freed > 0 {
					log.Printf("artifact cache sweep: reclaimed %d bytes", freed)
				}
			}
		}
	}()
}

// reconcileInterval is how often the durable tier is reconciled against the
// live deploy set after the initial startup pass.
const reconcileInterval = time.Hour

// defaultMaxUploadBytes bounds a CI upload's compressed body and its
// decompressed expansion out of the box, so an unauthenticated (auth-exempt by
// default) client can't stream an unbounded tar or a gzip bomb to disk. It is
// generous enough for real frontend/backend/artifact tars; deployments with
// larger legitimate artifacts raise --max-upload-bytes.
const defaultMaxUploadBytes int64 = 2 << 30 // 2 GiB

// startReconciler runs an initial reconcile pass, then repeats it on an
// interval. Returns immediately; the loop stops when ctx is cancelled. A gap
// (an artifact missing from the tier and not resident locally) is logged
// loudly — it is unrecoverable without a rebuild and blocks a serve-only node.
func startReconciler(ctx context.Context, r *reconcile.Reconciler) {
	run := func() {
		rep, err := r.Reconcile(ctx)
		if err != nil {
			log.Printf("reconcile: %v", err)
			return
		}
		log.Printf("reconcile: %s", rep)
		if rep.Gaps > 0 {
			// Cap the enumerated keys: the first pass after a repo set predates
			// the tier can surface many gaps at once, and an unbounded slice in
			// one log line is unreadable.
			shown := rep.GapKeys
			suffix := ""
			if len(shown) > 20 {
				suffix = fmt.Sprintf(" (+%d more)", len(shown)-20)
				shown = shown[:20]
			}
			log.Printf("reconcile: WARNING %d artifact(s) missing from the tier and not resident locally: %v%s",
				rep.Gaps, shown, suffix)
		}
	}
	go func() {
		run()
		t := time.NewTicker(reconcileInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
}

// hostOf extracts the host (without port) from a base URL, the default routable
// host for a worker's preview processes when --worker-host isn't given.
func hostOf(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return baseURL
}

// Fleet timings: how often the control node polls workers for capacity, how
// long a silent worker survives before placement treats it as gone, and how
// often the fleet-wide load signal is logged (the input a scale-out policy
// target-tracks).
const (
	fleetHeartbeatInterval = 10 * time.Second
	fleetStaleAfter        = 30 * time.Second
	fleetSignalInterval    = time.Minute
	// cloudPublishInterval is half the CloudWatch alarms' 60s period: two
	// datapoints per period means a tick that drifts across a minute boundary
	// can't leave a period empty (missing data reads as notBreaching, which
	// would sit on real demand for an extra minute), and demand is seen up to
	// 30s sooner on the coldest path — a user staring at a fleet of zero.
	cloudPublishInterval = 30 * time.Second
	// workerRegisterInterval is how often a self-registering worker re-announces
	// itself. Registration is idempotent; re-announcing lets a restarted control
	// node re-learn a worker that is already up (registrations aren't persisted).
	workerRegisterInterval = 20 * time.Second
)

// workerEndpoints returns the deduplicated worker-API base URLs for the control
// node, merging the single --worker-endpoint and the comma-separated
// --worker-endpoints.
func workerEndpoints(opts serveOptions) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(opts.workerEndpoint)
	for _, e := range strings.Split(opts.workerEndpoints, ",") {
		add(e)
	}
	return out
}

// parseReservedUpstreams turns "<label>=host:port" entries (from repeated
// --reserved-upstream flags or the comma-split env var) into a label→addr map,
// rejecting malformed entries, an empty label, or a duplicate label so a
// misconfiguration fails fast at startup rather than silently dropping a route.
func parseReservedUpstreams(entries []string) (map[string]string, error) {
	out := map[string]string{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		label, addr, ok := strings.Cut(e, "=")
		label = strings.ToLower(strings.TrimSpace(label))
		addr = strings.TrimSpace(addr)
		if !ok || label == "" || addr == "" {
			return nil, fmt.Errorf("--reserved-upstream %q: want <label>=host:port", e)
		}
		if strings.Contains(label, ".") {
			return nil, fmt.Errorf("--reserved-upstream %q: label must be a single DNS label (no dots)", e)
		}
		if _, dup := out[label]; dup {
			return nil, fmt.Errorf("--reserved-upstream: label %q set more than once", label)
		}
		out[label] = addr
	}
	return out, nil
}

// fleetStatsFn adapts a fleet registry to the api.Deps.FleetStats accessor,
// or returns nil when there is no fleet (single node), so the stats handler
// falls back to the local Manager.
func fleetStatsFn(reg *fleet.Registry) func() api.FleetSummary {
	if reg == nil {
		return nil
	}
	return func() api.FleetSummary {
		s := reg.Stat()
		return api.FleetSummary{
			Workers:    s.Workers,
			Running:    s.Running,
			Capacity:   s.Capacity,
			WarmHits:   s.WarmHits,
			ColdStarts: s.ColdStarts,
		}
	}
}

// startFleetSignal periodically logs the fleet's load ratio (committed warm
// slots ÷ capacity) — the signal a scale-out policy (e.g. a CloudWatch
// target-tracking alarm scraping this line, or a sidecar publishing it as a
// custom metric) drives autoscaling from. Returns immediately.
func startFleetSignal(ctx context.Context, reg *fleet.Registry) {
	go func() {
		t := time.NewTicker(fleetSignalInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				running, capacity := reg.Capacity()
				log.Printf("fleet: load=%.2f (%d/%d warm slots)", reg.LoadRatio(), running, capacity)
			}
		}
	}()
}

// isPublicBind reports whether a listen address binds all interfaces rather
// than a specific private one — an empty host or a wildcard IP. Used only to
// warn: the worker API is a remote-code-execution surface and should be
// firewalled to the control node, not exposed on every interface.
func isPublicBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return true
	}
	return false
}

// workerRegistrar adds workers to the fleet registry — the one place that turns
// an endpoint into a workerapi.Client with the control-DB SpecResolver attached.
// Shared by the static --worker-endpoint boot loop and the dynamic
// self-registration handler so a hand-listed and a self-registered worker join
// by exactly the same path. Satisfies workerapi.Registrar.
type workerRegistrar struct {
	reg    *fleet.Registry
	super  *supervise.Manager
	secret string
}

func (wr workerRegistrar) Register(endpoint, host, instanceID string) error {
	wc := workerapi.NewClient(endpoint, host, wr.secret, nil)
	// The worker has no artifact rows: every ensure carries the control-DB
	// resolved run spec.
	wc.SpecResolver = wr.super.ResolveWireSpec
	// And init results flow back: a successful backend ensure proves the
	// worker ran (or already had) the artifact's init, so record it here —
	// otherwise init_done_at never gets set on a control node that only
	// routes, and every fresh worker re-runs init on cold placement.
	wc.InitMarker = wr.super.AdoptRemoteInitDone
	// And the mirror: a worker-reported start failure on an ensure that
	// skipped init (because we told it InitDone=true) withdraws that record,
	// so the next ensure re-runs init. Init's effects can live in an external
	// service — a per-preview database someone reaped — where "done" quietly
	// stops being true.
	wc.InitRevoker = wr.super.RevokeInitDone
	// Add is idempotent for a known worker (they re-announce every ~20s);
	// only a genuinely new one is worth a log line.
	if wr.reg.Add(endpoint, instanceID, wc) {
		log.Printf("fleet: worker registered %s (processes at %s, instance %q)", endpoint, host, instanceID)
	}
	return nil
}

func (wr workerRegistrar) Deregister(endpoint string) error {
	wr.reg.Remove(endpoint)
	log.Printf("fleet: worker deregistered %s", endpoint)
	return nil
}

// deriveAdvertise resolves the worker-API base URL a self-registering worker
// tells the control node to dial back: the explicit --worker-advertise, else
// derived from a host-qualified --worker-listen (":9100" with no host can't be
// dialed remotely, so that case is an error the operator must resolve).
func deriveAdvertise(opts serveOptions) (string, error) {
	if opts.workerAdvertise != "" {
		return opts.workerAdvertise, nil
	}
	host, port, err := net.SplitHostPort(opts.workerListen)
	if err == nil && host != "" && host != "0.0.0.0" && host != "127.0.0.1" && host != "localhost" && host != "::" {
		return "http://" + net.JoinHostPort(host, port), nil
	}
	return "", fmt.Errorf("--control-endpoint requires --worker-advertise (this worker's routable worker-API URL); it cannot be derived from --worker-listen %q", opts.workerListen)
}

// startWorkerRegistration announces this worker to the control node immediately
// and every workerRegisterInterval, and sends a best-effort deregister when ctx
// is cancelled (graceful shutdown). Returns immediately.
func startWorkerRegistration(ctx context.Context, cc *workerapi.ControlClient, endpoint, host, instanceID string) {
	register := func() {
		rctx, cancel := context.WithTimeout(ctx, workerRegisterInterval)
		defer cancel()
		if err := cc.Register(rctx, endpoint, host, instanceID); err != nil {
			log.Printf("worker registration: %v (will retry)", err)
		}
	}
	go func() {
		register()
		t := time.NewTicker(workerRegisterInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				// Deregister with a fresh short-lived context — ctx is cancelled.
				dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := cc.Deregister(dctx, endpoint); err != nil {
					log.Printf("worker deregistration: %v", err)
				}
				cancel()
				return
			case <-t.C:
				register()
			}
		}
	}()
}
