package db

import (
	"errors"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession(Session{
		TokenHash:    "hash1",
		Scope:        "apex",
		GitHubLogin:  "octocat",
		GitHubUserID: 42,
		Email:        "octo@example.com",
		AvatarURL:    "https://example.com/a.png",
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == 0 || sess.CreatedAt == "" || sess.ExpiresAt == "" {
		t.Fatalf("expected id and timestamps, got %+v", sess)
	}

	got, err := s.GetSessionByTokenHash("apex", "hash1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GitHubLogin != "octocat" || got.GitHubUserID != 42 || got.Email != "octo@example.com" {
		t.Fatalf("mismatch: %+v", got)
	}

	byID, err := s.GetSessionByID(sess.ID)
	if err != nil || byID.TokenHash != "hash1" {
		t.Fatalf("by id: %v %+v", err, byID)
	}

	// A different scope must never match the same token hash.
	if _, err := s.GetSessionByTokenHash("preview", "hash1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong scope: want ErrNotFound, got %v", err)
	}
}

func TestSessionExpiredNotReturned(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateSession(Session{TokenHash: "h", Scope: "apex", GitHubLogin: "x", GitHubUserID: 1}, -time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSessionByTokenHash("apex", "h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for expired, got %v", err)
	}
	if err := s.DeleteExpiredSessions(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionDelete(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateSession(Session{TokenHash: "h", Scope: "apex", GitHubLogin: "x", GitHubUserID: 1}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSessionByTokenHash("apex", "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSessionByTokenHash("apex", "h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPreviewGrantSingleUse(t *testing.T) {
	s := newTestStore(t)
	apex, err := s.CreateSession(Session{TokenHash: "apexh", Scope: "apex", GitHubLogin: "x", GitHubUserID: 1}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePreviewGrant("code1", apex.ID, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := s.RedeemPreviewGrant("code1")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if got != apex.ID {
		t.Fatalf("want apex id %d, got %d", apex.ID, got)
	}
	// A code redeems exactly once.
	if _, err := s.RedeemPreviewGrant("code1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound on second redeem, got %v", err)
	}
}

func TestPreviewGrantExpired(t *testing.T) {
	s := newTestStore(t)
	apex, err := s.CreateSession(Session{TokenHash: "apexh", Scope: "apex", GitHubLogin: "x", GitHubUserID: 1}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePreviewGrant("code2", apex.ID, -time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemPreviewGrant("code2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for expired grant, got %v", err)
	}
	if err := s.DeleteExpiredPreviewGrants(); err != nil {
		t.Fatal(err)
	}
}
