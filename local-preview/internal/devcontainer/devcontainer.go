// Package devcontainer parses the subset of a target repository's
// devcontainer.json the orchestrator honors when defaulting builds and runs
// into the repo's dev container: the image, its named cache volumes, and the
// remote user's home (so toolchain caches resolve where the dev container
// put them). Like preview.toml, the file is read from the committed tree of
// the deployed sha, never from a checkout.
//
// Everything else — Dockerfile builds, features, runArgs, containerEnv,
// lifecycle commands — is deliberately ignored: honoring them faithfully
// requires the devcontainer CLI, and the goal here is only a warm, matching
// toolchain for build steps and server processes.
package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Config is the resolved subset of devcontainer.json. The zero value (empty
// Image) means "no usable devcontainer". It round-trips through JSON as part
// of stored run configs, so field order and tags are part of the contract.
type Config struct {
	// Image is the container image builds and runs default into.
	Image string `json:"image"`
	// Mounts are the devcontainer's named-volume mounts (bind mounts are
	// dropped — they reference paths personal to the developer's machine).
	// Sharing these volumes is what keeps preview builds warm: the dev
	// sandbox already populated them.
	Mounts []Mount `json:"mounts,omitempty"`
	// Home is the remote user's home directory, derived from remoteUser /
	// containerUser. Steps run with HOME set to it so toolchains (go, npm)
	// resolve their caches onto the mounted volumes.
	Home string `json:"home,omitempty"`
}

// Mount is one named-volume mount.
type Mount struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// raw mirrors the devcontainer.json keys we read. Mounts entries are either
// strings ("source=x,target=y,type=volume") or objects.
type rawConfig struct {
	Image         string            `json:"image"`
	Mounts        []json.RawMessage `json:"mounts"`
	RemoteUser    string            `json:"remoteUser"`
	ContainerUser string            `json:"containerUser"`
}

type rawMount struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// Parse resolves devcontainer.json content against the server's environment
// (${localEnv:VAR} expansion). An unparseable file is an error; a parseable
// file without a usable image (e.g. a Dockerfile build) yields a zero Config
// and no error — the caller falls back to the host and says why.
func Parse(data []byte) (Config, error) {
	return parse(data, os.LookupEnv)
}

func parse(data []byte, lookupEnv func(string) (string, bool)) (Config, error) {
	var raw rawConfig
	if err := json.Unmarshal(stripJSONC(data), &raw); err != nil {
		return Config{}, fmt.Errorf("parse devcontainer.json: %w", err)
	}
	image := expandVars(raw.Image, lookupEnv)
	if image == "" || hasUnresolvedVars(image) {
		return Config{}, nil
	}
	cfg := Config{Image: image}
	for _, m := range raw.Mounts {
		if mount, ok := volumeMount(m, lookupEnv); ok {
			cfg.Mounts = append(cfg.Mounts, mount)
		}
	}
	user := expandVars(raw.RemoteUser, lookupEnv)
	if user == "" {
		user = expandVars(raw.ContainerUser, lookupEnv)
	}
	switch {
	case user == "" || hasUnresolvedVars(user):
	case user == "root":
		cfg.Home = "/root"
	default:
		cfg.Home = "/home/" + user
	}
	return cfg, nil
}

// volumeNameRE is docker's local volume name grammar.
var volumeNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// volumeMount decodes one mounts entry (string or object form), keeping only
// resolvable named-volume mounts with absolute targets.
func volumeMount(entry json.RawMessage, lookupEnv func(string) (string, bool)) (Mount, bool) {
	var m rawMount
	var s string
	switch {
	case json.Unmarshal(entry, &s) == nil:
		for _, part := range strings.Split(s, ",") {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "source", "src":
				m.Source = v
			case "target", "dst", "destination":
				m.Target = v
			case "type":
				m.Type = v
			}
		}
	case json.Unmarshal(entry, &m) == nil:
	default:
		return Mount{}, false
	}
	if m.Type != "volume" {
		return Mount{}, false
	}
	m.Source = expandVars(m.Source, lookupEnv)
	m.Target = expandVars(m.Target, lookupEnv)
	if hasUnresolvedVars(m.Source) || hasUnresolvedVars(m.Target) {
		return Mount{}, false
	}
	if !volumeNameRE.MatchString(m.Source) || !strings.HasPrefix(m.Target, "/") {
		return Mount{}, false
	}
	return Mount{Source: m.Source, Target: m.Target}, true
}

// localEnvRE matches ${localEnv:VAR} and ${localEnv:VAR:default}.
var localEnvRE = regexp.MustCompile(`\$\{localEnv:([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?\}`)

// expandVars substitutes ${localEnv:...} references against the server's
// environment — the only devcontainer variable class that's meaningful
// server-side. Other variables (${localWorkspaceFolder}, …) are left intact
// for hasUnresolvedVars to reject.
func expandVars(s string, lookupEnv func(string) (string, bool)) string {
	return localEnvRE.ReplaceAllStringFunc(s, func(ref string) string {
		groups := localEnvRE.FindStringSubmatch(ref)
		if v, ok := lookupEnv(groups[1]); ok && v != "" {
			return v
		}
		return groups[2]
	})
}

func hasUnresolvedVars(s string) bool {
	return strings.Contains(s, "${")
}

// stripJSONC removes // and /* */ comments, then trailing commas —
// devcontainer.json is JSON-with-comments by spec. Two passes so a comment
// sitting between a trailing comma and its closer doesn't hide the comma.
func stripJSONC(data []byte) []byte {
	return stripTrailingCommas(stripComments(data))
}

func stripComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				out = append(out, data[i+1])
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++ // skip the '/' (loop increment skips the '*')
		default:
			out = append(out, c)
		}
	}
	return out
}

func stripTrailingCommas(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				out = append(out, data[i+1])
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
		}
		if c == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
