package session

import (
	"os"
	"path/filepath"
)

// claudeSettings is the contents of .claude/settings.local.json dropped into
// each worktree. Hooks call back into the kanban API with the session's
// active state so the ticket badge in the UI reflects what claude is doing,
// and to record the Claude Code session UUID so it can be `--resume`d after
// a container/Kanban restart.
//
// The hook commands rely on KANBAN_SESSION_ID and KANBAN_API_URL being
// injected into the session container's environment by the session manager.
// Failures are swallowed so the agent never blocks on a kanban outage. The
// SessionStart hook parses session_id from its stdin JSON via sed (no jq
// dependency assumed) — Claude Code passes the canonical UUID first in the
// payload object.
//
// PostToolUse reports "working" again once a tool has run. Without it a
// session that stopped for a permission prompt stays "awaiting_perm" for the
// rest of the turn after the user approves, and a second prompt in the same
// turn is indistinguishable from the first.
//
// writeClaudeSettings never overwrites a file it didn't write, so a
// hand-authored settings.local.json is left alone and opts out of these
// hooks. It does replace legacyClaudeSettings, the previous generated file.
const claudeSettings = claudeSettingsHead + claudeSettingsPostToolUse + claudeSettingsTail

// legacyClaudeSettings is what kanban wrote before the PostToolUse hook.
const legacyClaudeSettings = claudeSettingsHead + claudeSettingsTail

const claudeSettingsHead = `{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sid=$(sed -n 's/.*\"session_id\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p'); [ -n \"$sid\" ] && curl -fsS -m 2 -X PATCH -H 'Content-Type: application/json' -d \"{\\\"claude_session_id\\\":\\\"$sid\\\"}\" \"$KANBAN_API_URL/api/sessions/$KANBAN_SESSION_ID/claude-session\" >/dev/null 2>&1 || true"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -fsS -m 2 -X PATCH -H 'Content-Type: application/json' -d '{\"status\":\"working\"}' \"$KANBAN_API_URL/api/sessions/$KANBAN_SESSION_ID/status\" >/dev/null 2>&1 || true"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -fsS -m 2 -X PATCH -H 'Content-Type: application/json' -d '{\"status\":\"idle\"}' \"$KANBAN_API_URL/api/sessions/$KANBAN_SESSION_ID/status\" >/dev/null 2>&1 || true"
          }
        ]
      }
    ],
`

const claudeSettingsPostToolUse = `    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -fsS -m 2 -X PATCH -H 'Content-Type: application/json' -d '{\"status\":\"working\"}' \"$KANBAN_API_URL/api/sessions/$KANBAN_SESSION_ID/status\" >/dev/null 2>&1 || true"
          }
        ]
      }
    ],
`

const claudeSettingsTail = `    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -fsS -m 2 -X PATCH -H 'Content-Type: application/json' -d '{\"status\":\"awaiting_perm\"}' \"$KANBAN_API_URL/api/sessions/$KANBAN_SESSION_ID/status\" >/dev/null 2>&1 || true"
          }
        ]
      }
    ]
  }
}
`

// writeClaudeSettings writes .claude/settings.local.json into the worktree
// when no file is present, or when the file is byte-for-byte the one an
// older kanban generated. Anything else is left untouched, so hand-authored
// settings keep their file (and opt out of the features the hooks enable,
// e.g. automatic resume).
func writeClaudeSettings(worktreePath string) error {
	dir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.local.json")
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) != legacyClaudeSettings {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(claudeSettings), 0o644)
}
