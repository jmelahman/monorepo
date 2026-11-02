// Package lfs lets kanban check out and stage Git LFS files without git-lfs.
// See REGRESSIONS.md: "A mounted ~/.gitconfig can require git-lfs".
//
// A mounted ~/.gitconfig usually has `git lfs install`'s filter.lfs.process and
// filter.lfs.required=true, which fail every worktree of a repository tracking
// LFS files where git-lfs isn't installed. Git ignores filter.lfs.clean and
// smudge whenever process is set, and config can't unset it, so kanban takes
// its place: Install points filter.lfs.process at kanban's own executable, and
// main dispatches a leading `lfs-filter` here (see Main).
//
// Disabling the filter instead would check out pointers, and then `git add -A`
// would commit the real files' bytes as ordinary blobs in their place.
//
// Only the local object store is used: nothing is downloaded or uploaded, so an
// object that isn't here stays a pointer in the checkout, with a warning. Where
// git-lfs is installed, Install leaves it alone, so this never does worse.
package lfs

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Command is the leading argument main dispatches to Main.
const Command = "lfs-filter"

// Install has the git commands kanban runs from here on use it as their LFS
// filter, unless git-lfs is installed. It's set through the environment, which
// session containers don't inherit, and only matters to files that a
// repository's attributes mark filter=lfs.
func Install() {
	if _, err := exec.LookPath("git-lfs"); err == nil {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	configure(self)
}

// configure adds the filter to the config in the environment, after whatever is
// there already.
func configure(self string) {
	n, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	// Git runs a filter's process command through a shell.
	process := "'" + strings.ReplaceAll(self, "'", `'\''`) + "' " + Command
	_ = os.Setenv("GIT_CONFIG_KEY_"+strconv.Itoa(n), "filter.lfs.process")
	_ = os.Setenv("GIT_CONFIG_VALUE_"+strconv.Itoa(n), process)
	_ = os.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(n+1))
}

// Main serves git's long-running filter protocol on stdin and stdout, for the
// repository in the working directory, and returns the exit code.
func Main() int {
	if err := serve(os.Stdin, os.Stdout, &store{}); err != nil {
		fmt.Fprintln(os.Stderr, "kanban lfs-filter:", err)
		return 1
	}
	return 0
}

// maxPointer is the most a pointer can take, as git-lfs has it. Anything longer
// is content.
const maxPointer = 1024

var pointerRE = regexp.MustCompile(`\Aversion https://git-lfs\.github\.com/spec/v1\noid sha256:([0-9a-f]{64})\nsize (0|[1-9][0-9]*)\n\z`)

// parsePointer returns the object a pointer names. Only the canonical form
// counts, without extensions: anything else passes through as content. The oid
// is what keeps a crafted pointer's path inside the object store.
func parsePointer(b []byte) (oid string, size int64, ok bool) {
	if len(b) > maxPointer {
		return "", 0, false
	}
	m := pointerRE.FindSubmatch(b)
	if m == nil {
		return "", 0, false
	}
	size, err := strconv.ParseInt(string(m[2]), 10, 64)
	if err != nil {
		return "", 0, false
	}
	return string(m[1]), size, true
}

func formatPointer(oid string, size int64) string {
	return fmt.Sprintf("version https://git-lfs.github.com/spec/v1\noid sha256:%s\nsize %d\n", oid, size)
}

// store is the repository's LFS objects, found when first needed.
type store struct {
	dir string
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func (s *store) root() (string, error) {
	if s.dir != "" {
		return s.dir, nil
	}
	common, err := gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("find the git directory: %w", err)
	}
	dir := filepath.Join(common, "lfs")
	// Exit 1 is that it's unset.
	if v, err := gitOutput("config", "--get", "--path", "lfs.storage"); err == nil && v != "" {
		dir = v
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(common, dir)
		}
	}
	s.dir = dir
	return dir, nil
}

func (s *store) objectPath(oid string) (string, error) {
	root, err := s.root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "objects", oid[:2], oid[2:4], oid), nil
}

// open returns the object, or nil if it isn't here whole.
func (s *store) open(oid string, size int64) (*os.File, error) {
	path, err := s.objectPath(oid)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // a missing object is expected
	}
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || info.Size() != size {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// add stores content as an object and returns its pointer. The name is only
// taken once the whole file is there.
func (s *store) add(content io.Reader) (string, error) {
	root, err := s.root()
	if err != nil {
		return "", err
	}
	tmpDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(tmpDir, "kanban-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), content)
	if err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return "", err
	}
	oid := hex.EncodeToString(hash.Sum(nil))
	path, err := s.objectPath(oid)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return formatPointer(oid, size), nil
}

// The pkt-line format of git's protocols: four hex digits of length, counting
// themselves, then that much data. "0000" is a flush.
const maxPayload = 65516

type pktReader struct {
	r *bufio.Reader
}

// next returns the next packet's data, or flush true for a flush.
func (p pktReader) next() (data []byte, flush bool, err error) {
	var head [4]byte
	if _, err := io.ReadFull(p.r, head[:]); err != nil {
		return nil, false, err
	}
	n, err := strconv.ParseUint(string(head[:]), 16, 16)
	if err != nil {
		return nil, false, fmt.Errorf("bad packet length %q", head[:])
	}
	if n == 0 {
		return nil, true, nil
	}
	if n < 4 {
		return nil, false, fmt.Errorf("unexpected packet type %d", n)
	}
	data = make([]byte, n-4)
	if _, err := io.ReadFull(p.r, data); err != nil {
		return nil, false, unexpectedEOF(err)
	}
	return data, false, nil
}

func unexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// lines returns the text packets up to a flush. It's io.EOF only if git closed
// the pipe before any.
func (p pktReader) lines() ([]string, error) {
	var lines []string
	for {
		data, flush, err := p.next()
		if err != nil {
			if len(lines) > 0 {
				return nil, unexpectedEOF(err)
			}
			return nil, err
		}
		if flush {
			return lines, nil
		}
		lines = append(lines, strings.TrimSuffix(string(data), "\n"))
	}
}

// contentReader reads the data packets up to a flush as a stream. It must be
// read to its end for the next request to follow.
type contentReader struct {
	p    pktReader
	rest []byte
	done bool
	err  error
}

func (c *contentReader) Read(b []byte) (int, error) {
	for len(c.rest) == 0 {
		if c.err != nil {
			return 0, c.err
		}
		if c.done {
			return 0, io.EOF
		}
		data, flush, err := c.p.next()
		switch {
		case err != nil:
			c.err = unexpectedEOF(err)
		case flush:
			c.done = true
		default:
			c.rest = data
		}
	}
	n := copy(b, c.rest)
	c.rest = c.rest[n:]
	return n, nil
}

type pktWriter struct {
	w *bufio.Writer
}

func (p pktWriter) line(s string) error {
	return p.data([]byte(s + "\n"))
}

func (p pktWriter) data(b []byte) error {
	if _, err := fmt.Fprintf(p.w, "%04x", len(b)+4); err != nil {
		return err
	}
	_, err := p.w.Write(b)
	return err
}

func (p pktWriter) flush() error {
	if _, err := p.w.WriteString("0000"); err != nil {
		return err
	}
	return p.w.Flush()
}

// Write sends b as data packets.
func (p pktWriter) Write(b []byte) (int, error) {
	for sent := 0; sent < len(b); {
		n := min(len(b)-sent, maxPayload)
		if err := p.data(b[sent : sent+n]); err != nil {
			return sent, err
		}
		sent += n
	}
	return len(b), nil
}

// serve answers git until it closes the pipe. It returns an error only when the
// conversation breaks; a file it can't convert fails just that file.
func serve(in io.Reader, out io.Writer, s *store) error {
	r := pktReader{bufio.NewReader(in)}
	w := pktWriter{bufio.NewWriterSize(out, maxPayload+4)}
	if err := handshake(r, w); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	for {
		headers, err := r.lines()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var command, pathname string
		for _, h := range headers {
			key, value, _ := strings.Cut(h, "=")
			switch key {
			case "command":
				command = value
			case "pathname":
				pathname = value
			}
		}
		content := &contentReader{p: r}
		switch command {
		case "clean":
			err = clean(content, w, s, pathname)
		case "smudge":
			err = smudge(content, w, s, pathname)
		default:
			err = fileError(content, w, fmt.Errorf("unknown command %q", command), pathname)
		}
		if err != nil {
			return err
		}
	}
}

func handshake(r pktReader, w pktWriter) error {
	hello, err := r.lines()
	if err != nil {
		return unexpectedEOF(err)
	}
	if len(hello) < 2 || hello[0] != "git-filter-client" || !slices.Contains(hello[1:], "version=2") {
		return fmt.Errorf("unexpected greeting %q", hello)
	}
	for _, line := range []string{"git-filter-server", "version=2"} {
		if err := w.line(line); err != nil {
			return err
		}
	}
	if err := w.flush(); err != nil {
		return err
	}
	capabilities, err := r.lines()
	if err != nil {
		return unexpectedEOF(err)
	}
	for _, c := range []string{"capability=clean", "capability=smudge"} {
		if !slices.Contains(capabilities, c) {
			continue
		}
		if err := w.line(c); err != nil {
			return err
		}
	}
	return w.flush()
}

// fileError fails one file, once git has finished sending it, as it won't read
// a reply before then.
func fileError(content *contentReader, w pktWriter, cause error, pathname string) error {
	if _, err := io.Copy(io.Discard, content); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "kanban lfs-filter: %q: %v\n", pathname, cause)
	if err := w.line("status=error"); err != nil {
		return err
	}
	return w.flush()
}

// reply sends a successful conversion: its status, its content, and an empty
// list that keeps the status.
func reply(w pktWriter, content io.Reader) error {
	if err := w.line("status=success"); err != nil {
		return err
	}
	if err := w.flush(); err != nil {
		return err
	}
	// Packets can't be empty, and a read error can't be told apart from success
	// until the status that follows the content.
	_, copyErr := io.CopyBuffer(struct{ io.Writer }{w}, struct{ io.Reader }{content}, make([]byte, maxPayload))
	if err := w.flush(); err != nil {
		return err
	}
	if copyErr != nil {
		fmt.Fprintln(os.Stderr, "kanban lfs-filter:", copyErr)
		if err := w.line("status=error"); err != nil {
			return err
		}
	}
	return w.flush()
}

// clean turns content into a pointer, storing it as an object. A pointer stays
// as it is, so a checkout whose object was missing still shows no changes, and
// nothing stays nothing, as git-lfs has it.
func clean(content *contentReader, w pktWriter, s *store, pathname string) error {
	head := make([]byte, maxPointer+1)
	n, err := io.ReadFull(content, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if content.err != nil {
		return content.err
	}
	head = head[:n]
	if _, _, ok := parsePointer(head); ok || n == 0 {
		return reply(w, bytes.NewReader(head))
	}
	pointer, err := s.add(io.MultiReader(bytes.NewReader(head), content))
	if content.err != nil {
		return content.err
	}
	if err != nil {
		return fileError(content, w, err, pathname)
	}
	return reply(w, strings.NewReader(pointer))
}

// smudge turns a pointer into its object. Git holds the blob in memory anyway,
// so holding it here costs no more. Anything but a pointer passes through, and
// so does a pointer whose object isn't here: that's the difference from
// git-lfs, which would download it or fail.
func smudge(content *contentReader, w pktWriter, s *store, pathname string) error {
	blob, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	oid, size, ok := parsePointer(blob)
	if !ok {
		return reply(w, bytes.NewReader(blob))
	}
	if size == 0 {
		return reply(w, bytes.NewReader(nil))
	}
	f, err := s.open(oid, size)
	if err != nil {
		return fileError(content, w, err, pathname)
	}
	if f == nil {
		fmt.Fprintf(os.Stderr, "kanban lfs-filter: %q: LFS object %s isn't here, and kanban can't download it; leaving its pointer\n", pathname, oid)
		return reply(w, bytes.NewReader(blob))
	}
	defer func() { _ = f.Close() }()
	return reply(w, f)
}
