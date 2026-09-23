package db

import (
	"fmt"
	"time"
)

// secondsModifier renders a duration as a signed SQLite date modifier, e.g.
// "+3600 seconds" or "-60 seconds". Building the sign in Go (rather than
// hardcoding "+") keeps negative TTLs — used in tests to mint already-expired
// rows — valid instead of producing the malformed "+-60 seconds" that
// strftime turns into NULL.
func secondsModifier(ttl time.Duration) string {
	return fmt.Sprintf("%+d seconds", int64(ttl.Seconds()))
}

// Session is an authenticated login session. scope separates the dashboard
// trust boundary ('apex') from preview-subdomain access ('preview'); the two
// are never interchangeable. Only the sha256 hash of the opaque token is
// stored, so the table can't be replayed as a live credential if it leaks.
type Session struct {
	ID           int64
	TokenHash    string
	Scope        string
	GitHubLogin  string
	GitHubUserID int64
	Email        string
	AvatarURL    string
	CreatedAt    string
	ExpiresAt    string
}

// CreateSession stores a session whose token hashes to sess.TokenHash,
// expiring ttl from now by the database clock (matching created_at). Scope
// defaults to 'apex'. The returned Session carries the assigned id and the
// resolved timestamps.
func (s *Store) CreateSession(sess Session, ttl time.Duration) (Session, error) {
	if sess.Scope == "" {
		sess.Scope = "apex"
	}
	err := s.db.QueryRow(
		`INSERT INTO sessions
		   (token_hash, scope, github_login, github_user_id, email, avatar_url, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?,
		   strftime('%Y-%m-%dT%H:%M:%SZ', 'now', ?))
		 RETURNING id, created_at, expires_at`,
		sess.TokenHash, sess.Scope, sess.GitHubLogin, sess.GitHubUserID,
		sess.Email, sess.AvatarURL, secondsModifier(ttl),
	).Scan(&sess.ID, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// GetSessionByTokenHash returns the unexpired session with tokenHash in scope,
// or ErrNotFound when it is missing or already expired.
func (s *Store) GetSessionByTokenHash(scope, tokenHash string) (Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT id, token_hash, scope, github_login, github_user_id, email,
		        avatar_url, created_at, expires_at
		   FROM sessions
		  WHERE token_hash = ? AND scope = ?
		    AND expires_at > strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		tokenHash, scope,
	).Scan(&sess.ID, &sess.TokenHash, &sess.Scope, &sess.GitHubLogin,
		&sess.GitHubUserID, &sess.Email, &sess.AvatarURL,
		&sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return Session{}, mapNoRows(err)
	}
	return sess, nil
}

// GetSessionByID returns the unexpired session with the given id, or
// ErrNotFound. Used by the preview handshake to copy an apex session's
// identity onto a fresh preview-scoped session.
func (s *Store) GetSessionByID(id int64) (Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT id, token_hash, scope, github_login, github_user_id, email,
		        avatar_url, created_at, expires_at
		   FROM sessions
		  WHERE id = ?
		    AND expires_at > strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		id,
	).Scan(&sess.ID, &sess.TokenHash, &sess.Scope, &sess.GitHubLogin,
		&sess.GitHubUserID, &sess.Email, &sess.AvatarURL,
		&sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return Session{}, mapNoRows(err)
	}
	return sess, nil
}

// DeleteSessionByTokenHash removes a session (logout). A missing row is not an
// error.
func (s *Store) DeleteSessionByTokenHash(scope, tokenHash string) error {
	_, err := s.db.Exec(
		`DELETE FROM sessions WHERE token_hash = ? AND scope = ?`, tokenHash, scope)
	return err
}

// DeleteExpiredSessions drops sessions past their expiry.
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(
		`DELETE FROM sessions WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`)
	return err
}

// CreatePreviewGrant stores a single-use apex→preview handoff code (by hash)
// tied to an apex session, expiring ttl from now.
func (s *Store) CreatePreviewGrant(codeHash string, apexSessionID int64, ttl time.Duration) error {
	_, err := s.db.Exec(
		`INSERT INTO preview_grants (code_hash, apex_session_id, expires_at)
		 VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now', ?))`,
		codeHash, apexSessionID, secondsModifier(ttl))
	return err
}

// RedeemPreviewGrant consumes a grant code exactly once, returning the apex
// session id it was minted for. ErrNotFound means the code is unknown,
// expired, or already redeemed — the DELETE ... RETURNING makes checking and
// consuming a single atomic step, so a code can't be redeemed twice even under
// concurrent requests.
func (s *Store) RedeemPreviewGrant(codeHash string) (int64, error) {
	var apexSessionID int64
	err := s.db.QueryRow(
		`DELETE FROM preview_grants
		  WHERE code_hash = ?
		    AND expires_at > strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 RETURNING apex_session_id`,
		codeHash,
	).Scan(&apexSessionID)
	if err != nil {
		return 0, mapNoRows(err)
	}
	return apexSessionID, nil
}

// DeleteExpiredPreviewGrants drops grant codes past their expiry.
func (s *Store) DeleteExpiredPreviewGrants() error {
	_, err := s.db.Exec(
		`DELETE FROM preview_grants WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`)
	return err
}
