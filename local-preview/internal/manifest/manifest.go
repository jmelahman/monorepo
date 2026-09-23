// Package manifest parses preview.toml, the contract a target repository
// declares so the orchestrator can build and run it. The file is always read
// from a committed tree (git show <sha>:preview.toml), never a working dir.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Defaults applied when the optional backend timeouts are omitted.
const (
	DefaultStartTimeout = 20 * time.Second
	DefaultIdleTimeout  = 30 * time.Minute
	DefaultInitTimeout  = 2 * time.Minute
)

// Duration unmarshals from TOML strings like "20s" and round-trips through
// JSON as the same string form (run_config storage).
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for TOML decoding.
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON encodes as a duration string ("20s").
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts a duration string, or a raw nanosecond count for
// leniency.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		return d.UnmarshalText([]byte(s))
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("invalid duration %s", b)
	}
	*d = Duration(n)
	return nil
}

// Manifest is the parsed, validated preview.toml.
type Manifest struct {
	Frontend Frontend `toml:"frontend" json:"frontend"`
	Backend  Backend  `toml:"backend" json:"backend"`
	// Artifacts are prebuilt downloadable outputs ([artifacts.<name>]) —
	// CLIs and other binaries built per commit and served as file downloads
	// instead of run.
	Artifacts map[string]Artifact `toml:"artifacts" json:"artifacts,omitempty"`
	// Networks names existing external docker networks every containered
	// process joins — how previews reach shared dependencies the user runs
	// themselves (e.g. a target repo's own deps compose network).
	Networks []string `toml:"networks" json:"networks,omitempty"`
	// Devcontainer, when set to false, disables devcontainer discovery: sides
	// without an explicit image/run_image build and run on the host even if
	// the commit carries a usable .devcontainer/devcontainer.json. Unset
	// means true (discovery on).
	Devcontainer *bool `toml:"devcontainer" json:"devcontainer,omitempty"`
}

// DevcontainerEnabled reports whether devcontainer discovery applies.
func (m Manifest) DevcontainerEnabled() bool {
	return m.Devcontainer == nil || *m.Devcontainer
}

// Frontend describes how to hash, build, and serve the frontend. Build
// commands run with cwd <extracted-tree>/<Path>; Dist is relative to Path.
// Path doubles as the hash-partition root. Image, when set, runs the build
// steps inside that container image instead of on the host — being part of
// the manifest section, it feeds the artifact hash, so changing the image
// rebuilds.
//
// With Run set the frontend is a supervised server process instead of a
// static bundle: the whole built Path tree is published and all non-API
// traffic is proxied to the process. Dist is then unused; HealthPath is
// required. Run is templated with {port}; Env values support {port},
// {repo}, {hash}, and {backend_url}.
type Frontend struct {
	Path  string     `toml:"path" json:"path"`
	Build [][]string `toml:"build" json:"build"`
	Dist  string     `toml:"dist" json:"dist,omitempty"`
	Image string     `toml:"image" json:"image,omitempty"`

	Run          []string `toml:"run" json:"run,omitempty"`
	HealthPath   string   `toml:"health_path" json:"health_path,omitempty"`
	StartTimeout Duration `toml:"start_timeout" json:"start_timeout,omitempty"`
	IdleTimeout  Duration `toml:"idle_timeout" json:"idle_timeout,omitempty"`
	// RunImage, when set, runs the server process inside that container
	// image (artifact bind-mounted) instead of on the host.
	RunImage string            `toml:"run_image" json:"run_image,omitempty"`
	Env      map[string]string `toml:"env" json:"env,omitempty"`
}

// Backend describes how to hash, build, and run the backend. The hash covers
// entries under Path, minus the frontend's Path, minus Exclude patterns.
// Run is templated with {port} and {state_dir} at process start; Env values
// support {port}, {state_dir}, {repo}, and {hash}.
type Backend struct {
	Path    string     `toml:"path" json:"path"`
	Exclude []string   `toml:"exclude" json:"exclude,omitempty"`
	Build   [][]string `toml:"build" json:"build"`
	// Init steps run once per backend artifact — after its state dir is
	// provisioned, before the first Run — with {state_dir} templated. No
	// port exists yet, so {port} is rejected at parse time.
	Init         [][]string `toml:"init" json:"init,omitempty"`
	Run          []string   `toml:"run" json:"run"`
	HealthPath   string     `toml:"health_path" json:"health_path"`
	StartTimeout Duration   `toml:"start_timeout" json:"start_timeout,omitempty"`
	IdleTimeout  Duration   `toml:"idle_timeout" json:"idle_timeout,omitempty"`
	InitTimeout  Duration   `toml:"init_timeout" json:"init_timeout,omitempty"`
	// Image, when set, runs the build steps (not the server) inside that
	// container image instead of on the host.
	Image string `toml:"image" json:"image,omitempty"`
	// RunImage, when set, runs the server process inside that container
	// image (artifact and state dir bind-mounted) instead of on the host.
	RunImage string            `toml:"run_image" json:"run_image,omitempty"`
	Env      map[string]string `toml:"env" json:"env,omitempty"`
	// StripAPIPrefix removes the leading /api before proxying, for backends
	// whose routes aren't mounted under /api (onyx's nginx does the same).
	StripAPIPrefix bool `toml:"strip_api_prefix" json:"strip_api_prefix,omitempty"`
	// ExtraRoutes are additional path prefixes proxied to the backend
	// unstripped (e.g. /openapi.json, /auth/saml) alongside /api.
	ExtraRoutes []string `toml:"extra_routes" json:"extra_routes,omitempty"`
}

// Artifact describes a prebuilt downloadable output. Like the other sides,
// Path is the hash-partition root and the build cwd; the hash covers entries
// under Path, minus the frontend subtree, minus Exclude patterns — the same
// partition rule as the backend. Files are the build outputs published for
// download, relative to Path. Downloads are addressed by base name, so base
// names must be unique within one artifact. Nothing is ever run: there is
// no run command, health check, state dir, or env.
type Artifact struct {
	Path    string     `toml:"path" json:"path"`
	Exclude []string   `toml:"exclude" json:"exclude,omitempty"`
	Build   [][]string `toml:"build" json:"build"`
	Files   []string   `toml:"files" json:"files"`
	// Image, when set, runs the build steps inside that container image
	// instead of on the host.
	Image string `toml:"image" json:"image,omitempty"`
}

// ErrNoManifest marks a manifest source that isn't present: the file lacks
// the requested table. Callers with multiple candidate sources treat it as
// "try the next one".
var ErrNoManifest = errors.New("no preview manifest at this location")

// ParseAt decodes a manifest rooted at a top-level table of a larger TOML
// file — e.g. table "previews" inside a .kanban.toml — so embedders can
// offer repos a single config file. An empty table means the whole file is
// the manifest (preview.toml). Keys outside the table are ignored; unknown
// keys inside it are a hard error.
func ParseAt(data []byte, table string) (Manifest, error) {
	if table == "" {
		return Parse(data)
	}
	var doc map[string]toml.Primitive
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse manifest host file: %w", err)
	}
	prim, ok := doc[table]
	if !ok {
		return Manifest{}, ErrNoManifest
	}
	var m Manifest
	if err := md.PrimitiveDecode(prim, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse [%s]: %w", table, err)
	}
	var unknown []string
	for _, k := range md.Undecoded() {
		if len(k) > 0 && k[0] == table {
			unknown = append(unknown, k.String())
		}
	}
	if len(unknown) > 0 {
		return Manifest{}, fmt.Errorf("[%s]: unknown keys: %s", table, strings.Join(unknown, ", "))
	}
	if err := m.normalize(); err != nil {
		return Manifest{}, fmt.Errorf("[%s]: %w", table, err)
	}
	return m, nil
}

// Parse decodes and validates preview.toml content. Unknown keys are a hard
// error so typos surface at deploy time instead of silently changing nothing.
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	md, err := toml.Decode(string(data), &m)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse preview.toml: %w", err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return Manifest{}, fmt.Errorf("preview.toml: unknown keys: %s", strings.Join(keys, ", "))
	}
	if err := m.normalize(); err != nil {
		return Manifest{}, fmt.Errorf("preview.toml: %w", err)
	}
	return m, nil
}

func (m *Manifest) normalize() error {
	var err error
	if m.Frontend.Path, err = cleanRel("frontend.path", m.Frontend.Path); err != nil {
		return err
	}
	if err := validateSteps("frontend.build", m.Frontend.Build); err != nil {
		return err
	}
	if len(m.Frontend.Run) > 0 {
		// Process-mode frontend: a server, not a static bundle.
		if m.Frontend.Run[0] == "" {
			return fmt.Errorf("frontend.run must be a non-empty argv array")
		}
		if !strings.HasPrefix(m.Frontend.HealthPath, "/") {
			return fmt.Errorf("frontend.health_path is required with frontend.run and must start with %q", "/")
		}
		if m.Frontend.StartTimeout <= 0 {
			m.Frontend.StartTimeout = Duration(DefaultStartTimeout)
		}
		if m.Frontend.IdleTimeout <= 0 {
			m.Frontend.IdleTimeout = Duration(DefaultIdleTimeout)
		}
	} else {
		if m.Frontend.Dist, err = cleanRel("frontend.dist", m.Frontend.Dist); err != nil {
			return err
		}
	}
	if m.Backend.Path, err = cleanRel("backend.path", m.Backend.Path); err != nil {
		return err
	}
	if err := validateSteps("backend.build", m.Backend.Build); err != nil {
		return err
	}
	for i, step := range m.Backend.Init {
		if len(step) == 0 || step[0] == "" {
			return fmt.Errorf("backend.init[%d] must be a non-empty argv array", i)
		}
		for _, arg := range step {
			if strings.Contains(arg, "{port}") {
				return fmt.Errorf("backend.init[%d]: {port} is not available during init (no port is assigned until the server starts)", i)
			}
		}
	}
	if len(m.Backend.Run) == 0 || m.Backend.Run[0] == "" {
		return fmt.Errorf("backend.run is required")
	}
	if !strings.HasPrefix(m.Backend.HealthPath, "/") {
		return fmt.Errorf("backend.health_path must start with %q", "/")
	}
	if m.Backend.StartTimeout <= 0 {
		m.Backend.StartTimeout = Duration(DefaultStartTimeout)
	}
	if m.Backend.IdleTimeout <= 0 {
		m.Backend.IdleTimeout = Duration(DefaultIdleTimeout)
	}
	if m.Backend.InitTimeout <= 0 {
		m.Backend.InitTimeout = Duration(DefaultInitTimeout)
	}
	for _, r := range m.Backend.ExtraRoutes {
		if !strings.HasPrefix(r, "/") {
			return fmt.Errorf("backend.extra_routes: %q must start with %q", r, "/")
		}
	}
	for name, a := range m.Artifacts {
		if err := normalizeArtifact(name, &a); err != nil {
			return err
		}
		m.Artifacts[name] = a
	}
	if err := validateEnv("frontend.env", m.Frontend.Env, frontendPlaceholders); err != nil {
		return err
	}
	if err := validateEnv("backend.env", m.Backend.Env, backendPlaceholders); err != nil {
		return err
	}
	if envReferences(m.Frontend.Env, "{backend_url}") {
		if len(m.Frontend.Run) == 0 {
			return fmt.Errorf("frontend.env: {backend_url} requires frontend.run")
		}
		if (m.Frontend.RunImage == "") != (m.Backend.RunImage == "") {
			return fmt.Errorf("frontend.env: {backend_url} requires both sides on the same runtime (both run_image or neither)")
		}
	}
	return nil
}

// Artifact names appear in download URLs and on-disk layouts; keep them to
// the same lowercase-label shape as repo names.
var artifactNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func normalizeArtifact(name string, a *Artifact) error {
	field := "artifacts." + name
	if !artifactNameRE.MatchString(name) {
		return fmt.Errorf("artifacts: name %q must be a lowercase label (letters, digits, inner hyphens)", name)
	}
	var err error
	if a.Path, err = cleanRel(field+".path", a.Path); err != nil {
		return err
	}
	if err := validateSteps(field+".build", a.Build); err != nil {
		return err
	}
	if len(a.Files) == 0 {
		return fmt.Errorf("%s.files is required", field)
	}
	byBase := map[string]string{}
	for i, f := range a.Files {
		cf, err := cleanRel(fmt.Sprintf("%s.files[%d]", field, i), f)
		if err != nil {
			return err
		}
		if cf == "." {
			return fmt.Errorf("%s.files[%d] must name a file, not %q", field, i, ".")
		}
		if prev, dup := byBase[path.Base(cf)]; dup {
			return fmt.Errorf("%s.files: %q and %q share a base name (downloads are addressed by base name)", field, prev, cf)
		}
		byBase[path.Base(cf)] = cf
		a.Files[i] = cf
	}
	return nil
}

// Env value placeholders, expanded at process start (never at hash time —
// the literal string is what's hashed). {hash} is the side's own artifact
// hash: processes are shared per artifact across deploys, so isolation
// keyed on {hash} follows artifact identity the way state dirs do.
var (
	frontendPlaceholders = map[string]bool{"{port}": true, "{repo}": true, "{hash}": true, "{backend_url}": true}
	backendPlaceholders  = map[string]bool{"{port}": true, "{repo}": true, "{hash}": true, "{state_dir}": true}
)

var placeholderRE = regexp.MustCompile(`\{[a-z_]+\}`)

func validateEnv(field string, env map[string]string, allowed map[string]bool) error {
	for k, v := range env {
		if k == "" {
			return fmt.Errorf("%s: empty variable name", field)
		}
		for _, tok := range placeholderRE.FindAllString(v, -1) {
			if !allowed[tok] {
				return fmt.Errorf("%s.%s: unknown placeholder %s", field, k, tok)
			}
		}
	}
	return nil
}

func envReferences(env map[string]string, placeholder string) bool {
	for _, v := range env {
		if strings.Contains(v, placeholder) {
			return true
		}
	}
	return false
}

// cleanRel normalizes a repo-relative slash path and rejects anything that
// escapes the repo root. "." is allowed (the whole tree).
func cleanRel(field, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	cp := path.Clean(strings.ReplaceAll(p, "\\", "/"))
	if strings.HasPrefix(cp, "/") || cp == ".." || strings.HasPrefix(cp, "../") {
		return "", fmt.Errorf("%s must be a relative path inside the repo", field)
	}
	return cp, nil
}

func validateSteps(field string, steps [][]string) error {
	if len(steps) == 0 {
		return fmt.Errorf("%s is required", field)
	}
	for i, step := range steps {
		if len(step) == 0 || step[0] == "" {
			return fmt.Errorf("%s[%d] must be a non-empty argv array", field, i)
		}
	}
	return nil
}
