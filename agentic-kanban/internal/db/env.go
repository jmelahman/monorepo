package db

import (
	"context"
	"log"

	"github.com/jmelahman/kanban/internal/secrets"
)

// Board env vars are write-only secrets: values are AES-GCM encrypted by the
// Store's envCipher before they touch disk, and only GetBoardEnvVars (used at
// session launch) ever decrypts them. API surfaces list key names only.

// ListBoardEnvVarKeys returns the sorted env var key names for a board.
// Values are intentionally not returned.
func (s *Store) ListBoardEnvVarKeys(ctx context.Context, boardID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key FROM board_env_vars WHERE board_id = ? ORDER BY key`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetBoardEnvVars returns the decrypted key→value map for a board. Only the
// session launch path should call this; values must never reach an API
// response. Rows that fail to decrypt (e.g. the key file was replaced) are
// skipped with a warning rather than failing the whole call.
func (s *Store) GetBoardEnvVars(ctx context.Context, boardID int64) (map[string]string, error) {
	if s.envCipher == nil {
		return nil, secrets.ErrNoCipher
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM board_env_vars WHERE board_id = ? ORDER BY key`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vars := map[string]string{}
	for rows.Next() {
		var k, ct string
		if err := rows.Scan(&k, &ct); err != nil {
			return nil, err
		}
		v, err := s.envCipher.Decrypt(ct)
		if err != nil {
			log.Printf("board %d env var %s: cannot decrypt (key file changed?); skipping: %v", boardID, k, err)
			continue
		}
		vars[k] = v
	}
	return vars, rows.Err()
}

// SetBoardEnvVar encrypts value and upserts it under (boardID, key).
func (s *Store) SetBoardEnvVar(ctx context.Context, boardID int64, key, value string) error {
	if s.envCipher == nil {
		return secrets.ErrNoCipher
	}
	ct, err := s.envCipher.Encrypt(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO board_env_vars (board_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(board_id, key) DO UPDATE SET value = excluded.value`,
		boardID, key, ct)
	return err
}

// DeleteBoardEnvVar removes a key from a board. Deleting a missing key is a
// no-op so unset stays idempotent.
func (s *Store) DeleteBoardEnvVar(ctx context.Context, boardID int64, key string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM board_env_vars WHERE board_id = ? AND key = ?`, boardID, key)
	return err
}
