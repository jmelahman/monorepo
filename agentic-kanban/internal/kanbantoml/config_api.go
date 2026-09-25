package kanbantoml

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Kind classifies a config key so callers can validate, coerce, and render it
// without hard-coding per-key types. Top-level keys are scalars, arrays, maps,
// or arrays-of-tables; the nested fields of table entries (TaskEntry,
// BuildCopBoard) are validated by unmarshaling into their Go structs.
type Kind string

const (
	KindBool        Kind = "bool"
	KindString      Kind = "string"
	KindStringArray Kind = "string_array"
	KindStringMap   Kind = "string_map"
	KindTableArray  Kind = "table_array"
)

// KeySpec describes one configurable key. Validate, when non-nil, runs on the
// normalized Go value (e.g. a parsed duration string or a []TaskEntry) and
// returns a user-facing error. Validations that depend on other packages
// (e.g. harness.IsKnown) live in the API handler to avoid import cycles.
type KeySpec struct {
	Key      string
	Kind     Kind
	Validate func(v any) error
}

// schema is the ordered registry of every writable config key. Ordering drives
// `config list` output; keep it grouped by section.
var schema = []KeySpec{
	{Key: "harness.id", Kind: KindString},

	{Key: "sync.allow_rebase", Kind: KindBool},
	{Key: "sync.allow_merge", Kind: KindBool},

	{Key: "merge.allow_merge_commit", Kind: KindBool},
	{Key: "merge.allow_squash", Kind: KindBool},
	{Key: "merge.allow_rebase", Kind: KindBool},
	{Key: "merge.ai_commit_message", Kind: KindBool},
	{Key: "merge.default_strategy", Kind: KindString, Validate: validateMergeStrategy},

	{Key: "github.auto_move", Kind: KindBool},
	{Key: "github.draft_column", Kind: KindString},
	{Key: "github.review_column", Kind: KindString},
	{Key: "github.done_column", Kind: KindString},
	{Key: "github.closed_column", Kind: KindString},

	{Key: "worktrees.root", Kind: KindString},

	{Key: "git.sign_commits", Kind: KindBool},

	{Key: "plans.dir", Kind: KindString},

	{Key: "branches.prefix", Kind: KindString},

	{Key: "devcontainer.image", Kind: KindString},
	{Key: "devcontainer.run_args", Kind: KindStringArray},
	{Key: "devcontainer.mounts", Kind: KindStringArray},
	{Key: "devcontainer.container_env", Kind: KindStringMap},
	{Key: "devcontainer.docker_socket", Kind: KindBool},
	{Key: "devcontainer.claude_config", Kind: KindBool},

	{Key: "errors.enabled", Kind: KindBool},
	{Key: "errors.board_name", Kind: KindString},

	{Key: "buildcop.enabled", Kind: KindBool},
	{Key: "buildcop.interval", Kind: KindString, Validate: validateInterval},
	{Key: "buildcop.boards", Kind: KindTableArray, Validate: validateBoards},

	{Key: "dev_toolbar.enabled", Kind: KindBool},

	{Key: "task", Kind: KindTableArray, Validate: validateTasks},
}

// Schema returns the ordered list of writable config keys.
func Schema() []KeySpec {
	out := make([]KeySpec, len(schema))
	copy(out, schema)
	return out
}

// Lookup resolves a dotted key to its spec. For a StringMap key it also
// accepts a single-entry address (e.g. "devcontainer.container_env.FOO"),
// returning the spec and the trailing map key ("FOO"). mapSubkey is "" for an
// exact key match.
func Lookup(key string) (spec KeySpec, mapSubkey string, ok bool) {
	for _, s := range schema {
		if s.Key == key {
			return s, "", true
		}
	}
	for _, s := range schema {
		if s.Kind == KindStringMap && strings.HasPrefix(key, s.Key+".") {
			sub := strings.TrimPrefix(key, s.Key+".")
			if sub != "" && !strings.Contains(sub, ".") {
				return s, sub, true
			}
		}
	}
	return KeySpec{}, "", false
}

func validateInterval(v any) error {
	s, _ := v.(string)
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d < time.Second {
		return fmt.Errorf("interval must be at least 1s")
	}
	return nil
}

// MergeStrategies is the set of strategies `POST /api/tickets/{id}/merge`
// accepts, in the order they are offered. Exported so the API layer and the
// config validator agree on one list.
var MergeStrategies = []string{"merge-commit", "squash", "rebase"}

func validateMergeStrategy(v any) error {
	s, _ := v.(string)
	for _, valid := range MergeStrategies {
		if s == valid {
			return nil
		}
	}
	return fmt.Errorf("merge.default_strategy must be one of %s", strings.Join(MergeStrategies, ", "))
}

func validateTasks(v any) error {
	ts, _ := v.([]TaskEntry)
	for i, t := range ts {
		if strings.TrimSpace(t.Label) == "" {
			return fmt.Errorf("task[%d]: label is required", i)
		}
	}
	return nil
}

func validateBoards(v any) error {
	bs, _ := v.([]BuildCopBoard)
	for i, b := range bs {
		if b.RepoPath == nil || strings.TrimSpace(*b.RepoPath) == "" {
			return fmt.Errorf("buildcop.boards[%d]: repo_path is required", i)
		}
	}
	return nil
}

func typeErr(key, want string) error {
	return fmt.Errorf("%s expects a %s", key, want)
}

// NormalizeValue validates a raw JSON value for the given dotted key and
// converts it to a Go value suitable for writing via go-toml (bool, string,
// []string, map[string]string, []TaskEntry, []BuildCopBoard, or a string for a
// single map entry). A JSON null is rejected — callers clear keys via unset.
func NormalizeValue(key string, raw json.RawMessage) (any, error) {
	spec, sub, ok := Lookup(key)
	if !ok {
		return nil, fmt.Errorf("unknown config key %q", key)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("null value for %q; use unset to clear it", key)
	}
	if sub != "" {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, typeErr(key, "string")
		}
		return s, nil
	}

	var val any
	switch spec.Kind {
	case KindBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, typeErr(key, "boolean")
		}
		val = b
	case KindString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, typeErr(key, "string")
		}
		val = s
	case KindStringArray:
		var a []string
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, typeErr(key, "array of strings")
		}
		val = a
	case KindStringMap:
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, typeErr(key, "object of strings")
		}
		val = m
	case KindTableArray:
		switch key {
		case "task":
			var t []TaskEntry
			if err := json.Unmarshal(raw, &t); err != nil {
				return nil, typeErr(key, "array of {label, container_port}")
			}
			val = t
		case "buildcop.boards":
			var b []BuildCopBoard
			if err := json.Unmarshal(raw, &b); err != nil {
				return nil, typeErr(key, "array of buildcop board objects")
			}
			val = b
		default:
			return nil, fmt.Errorf("unsupported table key %q", key)
		}
	default:
		return nil, fmt.Errorf("unsupported kind for %q", key)
	}

	if spec.Validate != nil {
		if err := spec.Validate(val); err != nil {
			return nil, err
		}
	}
	return val, nil
}

// CoerceCLIValue turns a CLI string argument into a Go value for the given
// key. Booleans accept true/false; strings pass through; collections expect a
// JSON document (e.g. '["--a"]' or '[{"label":"web","container_port":3000}]').
// A single map entry (devcontainer.container_env.FOO) takes a bare string.
func CoerceCLIValue(key, raw string) (any, error) {
	spec, sub, ok := Lookup(key)
	if !ok {
		return nil, fmt.Errorf("unknown config key %q", key)
	}
	if sub != "" {
		return raw, nil
	}
	switch spec.Kind {
	case KindBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%s expects true or false, got %q", key, raw)
		}
		return b, nil
	case KindString:
		return raw, nil
	case KindStringArray, KindStringMap, KindTableArray:
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("%s expects a JSON value: %w", key, err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported kind for %q", key)
	}
}

// GetValue returns the value of a dotted key from a loaded File and whether it
// is set. Collection values are returned as their typed slices/maps. Map
// sub-keys (devcontainer.container_env.FOO) are not resolved here — callers
// index the returned map themselves.
func GetValue(f File, key string) (any, bool) {
	switch key {
	case "harness.id":
		if f.Harness != nil && f.Harness.ID != nil {
			return *f.Harness.ID, true
		}
	case "sync.allow_rebase":
		if f.Sync != nil && f.Sync.AllowRebase != nil {
			return *f.Sync.AllowRebase, true
		}
	case "sync.allow_merge":
		if f.Sync != nil && f.Sync.AllowMerge != nil {
			return *f.Sync.AllowMerge, true
		}
	case "merge.allow_merge_commit":
		if f.Merge != nil && f.Merge.AllowMergeCommit != nil {
			return *f.Merge.AllowMergeCommit, true
		}
	case "merge.allow_squash":
		if f.Merge != nil && f.Merge.AllowSquash != nil {
			return *f.Merge.AllowSquash, true
		}
	case "merge.allow_rebase":
		if f.Merge != nil && f.Merge.AllowRebase != nil {
			return *f.Merge.AllowRebase, true
		}
	case "merge.ai_commit_message":
		if f.Merge != nil && f.Merge.AICommitMessage != nil {
			return *f.Merge.AICommitMessage, true
		}
	case "merge.default_strategy":
		if f.Merge != nil && f.Merge.DefaultStrategy != nil {
			return *f.Merge.DefaultStrategy, true
		}
	case "github.auto_move":
		if f.GitHub != nil && f.GitHub.AutoMove != nil {
			return *f.GitHub.AutoMove, true
		}
	case "github.draft_column":
		if f.GitHub != nil && f.GitHub.DraftColumn != nil {
			return *f.GitHub.DraftColumn, true
		}
	case "github.review_column":
		if f.GitHub != nil && f.GitHub.ReviewColumn != nil {
			return *f.GitHub.ReviewColumn, true
		}
	case "github.done_column":
		if f.GitHub != nil && f.GitHub.DoneColumn != nil {
			return *f.GitHub.DoneColumn, true
		}
	case "github.closed_column":
		if f.GitHub != nil && f.GitHub.ClosedColumn != nil {
			return *f.GitHub.ClosedColumn, true
		}
	case "worktrees.root":
		if f.Worktrees != nil && f.Worktrees.Root != nil {
			return *f.Worktrees.Root, true
		}
	case "git.sign_commits":
		if f.Git != nil && f.Git.SignCommits != nil {
			return *f.Git.SignCommits, true
		}
	case "plans.dir":
		if f.Plans != nil && f.Plans.Dir != nil {
			return *f.Plans.Dir, true
		}
	case "branches.prefix":
		if f.Branches != nil && f.Branches.Prefix != nil {
			return *f.Branches.Prefix, true
		}
	case "devcontainer.image":
		if f.Devcontainer != nil && f.Devcontainer.Image != nil {
			return *f.Devcontainer.Image, true
		}
	case "devcontainer.run_args":
		if f.Devcontainer != nil && f.Devcontainer.RunArgs != nil {
			return f.Devcontainer.RunArgs, true
		}
	case "devcontainer.mounts":
		if f.Devcontainer != nil && f.Devcontainer.Mounts != nil {
			return f.Devcontainer.Mounts, true
		}
	case "devcontainer.container_env":
		if f.Devcontainer != nil && f.Devcontainer.ContainerEnv != nil {
			return f.Devcontainer.ContainerEnv, true
		}
	case "devcontainer.docker_socket":
		if f.Devcontainer != nil && f.Devcontainer.DockerSocket != nil {
			return *f.Devcontainer.DockerSocket, true
		}
	case "devcontainer.claude_config":
		if f.Devcontainer != nil && f.Devcontainer.ClaudeConfig != nil {
			return *f.Devcontainer.ClaudeConfig, true
		}
	case "errors.enabled":
		if f.Errors != nil && f.Errors.Enabled != nil {
			return *f.Errors.Enabled, true
		}
	case "errors.board_name":
		if f.Errors != nil && f.Errors.BoardName != nil {
			return *f.Errors.BoardName, true
		}
	case "buildcop.enabled":
		if f.BuildCop != nil && f.BuildCop.Enabled != nil {
			return *f.BuildCop.Enabled, true
		}
	case "buildcop.interval":
		if f.BuildCop != nil && f.BuildCop.Interval != nil {
			return *f.BuildCop.Interval, true
		}
	case "buildcop.boards":
		if f.BuildCop != nil && f.BuildCop.Boards != nil {
			return f.BuildCop.Boards, true
		}
	case "dev_toolbar.enabled":
		if f.DevToolbar != nil && f.DevToolbar.Enabled != nil {
			return *f.DevToolbar.Enabled, true
		}
	case "task":
		if f.Tasks != nil {
			return f.Tasks, true
		}
	}
	return nil, false
}

// LoadLayers reads the project file (for repoPath) and the user file
// separately, without merging, so callers can attribute each effective value
// to the layer that set it. Either layer may be empty.
func LoadLayers(repoPath string) (project, user File) {
	project = readFileAt(ProjectPath(repoPath))
	if path, err := UserPath(); err == nil {
		user = readFileAt(path)
	}
	return project, user
}

// EditFile applies dotted-key sets and unsets to a TOML file, preserving every
// other key already present. Set values may be scalars, []string,
// map[string]string, or typed table slices — go-toml marshals them via their
// struct tags. Unset removes the leaf and prunes any sections it leaves empty;
// an empty result deletes the file. Writes are atomic (temp file + rename) and
// serialized per path so concurrent callers (e.g. /api/settings and
// /api/config) never clobber each other.
func EditFile(path string, sets map[string]any, unsets []string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config file path is empty")
	}
	path = filepath.Clean(path)

	mu := fileMutex(path)
	mu.Lock()
	defer mu.Unlock()

	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	for key, val := range sets {
		setPath(root, strings.Split(key, "."), val)
	}
	for _, key := range unsets {
		unsetPath(root, strings.Split(key, "."))
	}

	if len(root) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := toml.Marshal(root)
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

// setPath assigns value at the nested map location named by segs, creating
// intermediate maps as needed. A single segment writes a top-level key (e.g.
// "task"). A non-map intermediate is replaced with a fresh map.
func setPath(root map[string]any, segs []string, value any) {
	m := root
	for _, s := range segs[:len(segs)-1] {
		next, ok := m[s].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[s] = next
		}
		m = next
	}
	m[segs[len(segs)-1]] = value
}

// unsetPath removes the leaf named by segs and prunes any ancestor maps left
// empty by the removal.
func unsetPath(root map[string]any, segs []string) {
	if len(segs) == 1 {
		delete(root, segs[0])
		return
	}
	chain := []map[string]any{root}
	m := root
	for _, s := range segs[:len(segs)-1] {
		next, ok := m[s].(map[string]any)
		if !ok {
			return // nothing to unset
		}
		chain = append(chain, next)
		m = next
	}
	delete(m, segs[len(segs)-1])
	for i := len(chain) - 1; i >= 1; i-- {
		if len(chain[i]) == 0 {
			delete(chain[i-1], segs[i-1])
		} else {
			break
		}
	}
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

var fileMutexes sync.Map // path -> *sync.Mutex

func fileMutex(path string) *sync.Mutex {
	m, _ := fileMutexes.LoadOrStore(path, &sync.Mutex{})
	return m.(*sync.Mutex)
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
