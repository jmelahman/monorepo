package db

import (
	"errors"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetSetting("retention"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing setting: want ErrNotFound, got %v", err)
	}
	if err := s.SetSetting("retention", `{"max_age_days":30}`); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSetting("retention")
	if err != nil {
		t.Fatal(err)
	}
	if v != `{"max_age_days":30}` {
		t.Fatalf("got %q", v)
	}
	// Overwrite replaces the value.
	if err := s.SetSetting("retention", `{"max_age_days":7}`); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.GetSetting("retention"); v != `{"max_age_days":7}` {
		t.Fatalf("after overwrite: got %q", v)
	}
}

func TestBranchAliasDeployIDs(t *testing.T) {
	s := newTestStore(t)
	ids, err := s.BranchAliasDeployIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("empty table: got %v", ids)
	}
	repo, err := s.CreateRepo("web", "/src/web", "/data/web.git", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.CreateDeploy(repo.ID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBranchAlias(repo.ID, "main", d.ID); err != nil {
		t.Fatal(err)
	}
	ids, err = s.BranchAliasDeployIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !ids[d.ID] || len(ids) != 1 {
		t.Fatalf("got %v, want {%d}", ids, d.ID)
	}

	// Re-pointing the alias replaces the target rather than adding a row.
	d2, err := s.CreateDeploy(repo.ID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBranchAlias(repo.ID, "main", d2.ID); err != nil {
		t.Fatal(err)
	}
	ids, err = s.BranchAliasDeployIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !ids[d2.ID] || len(ids) != 1 {
		t.Fatalf("after re-point: got %v, want {%d}", ids, d2.ID)
	}
}
