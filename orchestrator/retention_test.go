package orchestrator

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRetentionAndStorage(t *testing.T) {
	src := newSourceRepo(t)
	o, err := New(Options{
		DataDir: filepath.Join(t.TempDir(), "previews"),
		Addr:    ":8080",
		Runner:  &recordingRunner{},
		// Sweeps are driven explicitly here; a concurrent background pass
		// would race the assertions below.
		RetentionInterval: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	ctx := context.Background()

	if _, err := o.RegisterRepo(ctx, "demo", src); err != nil {
		t.Fatal(err)
	}
	dep, err := o.RequestDeploy(ctx, "demo", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	ready := waitReady(t, o, dep.ID)

	// The default policy evicts nothing.
	policy, err := o.RetentionPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy != (RetentionPolicy{}) {
		t.Fatalf("default policy = %+v, want zero", policy)
	}

	rep, err := o.Storage()
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalBytes <= 0 {
		t.Fatalf("TotalBytes = %d, want > 0 after a ready deploy", rep.TotalBytes)
	}
	if len(rep.Repos) != 1 || rep.Repos[0].Repo != "demo" {
		t.Fatalf("Repos = %+v, want one entry for demo", rep.Repos)
	}
	if rep.Repos[0].Deploys != 1 || rep.Repos[0].EvictedDeploys != 0 {
		t.Fatalf("deploy counts = %d active / %d evicted, want 1 / 0",
			rep.Repos[0].Deploys, rep.Repos[0].EvictedDeploys)
	}
	if rep.Repos[0].ArtifactsBytes <= 0 || rep.Repos[0].MirrorBytes <= 0 {
		t.Fatalf("repo usage = %+v, want artifacts and mirror accounted", rep.Repos[0])
	}

	// Negative limits are rejected; the stored policy is unchanged.
	if err := o.SetRetentionPolicy(RetentionPolicy{MaxAgeDays: -1}); err == nil {
		t.Fatal("negative MaxAgeDays should be rejected")
	}

	if err := o.SetRetentionPolicy(RetentionPolicy{MaxDeploysPerRepo: 1, MaxAgeDays: 7}); err != nil {
		t.Fatal(err)
	}
	got, err := o.RetentionPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got != (RetentionPolicy{MaxDeploysPerRepo: 1, MaxAgeDays: 7}) {
		t.Fatalf("policy round-trip = %+v", got)
	}

	// A repo always keeps its newest ready deploy, so this sweep evicts
	// nothing even though the count limit is 1 and one deploy exists.
	res, err := o.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 0 {
		t.Fatalf("Evicted = %+v, want the sole ready deploy protected", res.Evicted)
	}
	if res.Policy != got {
		t.Fatalf("result policy = %+v, want %+v", res.Policy, got)
	}
	if d, err := o.Deploy(ready.ID); err != nil || d.Status != StatusReady {
		t.Fatalf("deploy after sweep = %+v, %v", d, err)
	}
}
