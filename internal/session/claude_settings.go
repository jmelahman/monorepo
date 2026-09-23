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
// writeClaudeSettings only writes when no file exists, so any pre-existing
// settings.local.json (hand-authored or shipped before the resume feature)
// is left alone and opts out of resume.
const claudeSettings = `{
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
    "Notification": [
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
// when no file is already present. Any existing file is left untouched, so
// hand-authored settings and worktrees that pre-date newer hook wiring keep
// their original file (and opt out of any features those newer hooks
// enabled, e.g. automatic resume).
func writeClaudeSettings(worktreePath string) error {
	dir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.local.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(claudeSettings), 0o644)
}
