package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNaturalOrder(t *testing.T) {
	names := []string{"10 Ten", "2 - Two", "01. One", "Intro", "3.Three"}
	slices.SortFunc(names, natural)
	want := []string{"01. One", "2 - Two", "3.Three", "10 Ten", "Intro"}
	if !slices.Equal(names, want) {
		t.Fatalf("got %q, want %q", names, want)
	}
}

func TestWalkLibraryMergesSingles(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"A/Loose.mp3", "A/Singles/Kept.mp3", "A/Album/01 First.mp3", "A/cover.jpg"} {
		f := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	artists, err := walkLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 || len(artists[0].Albums) != 2 {
		t.Fatalf("want one artist with two albums, got %+v", artists)
	}
	singles := artists[0].Albums[1]
	if singles.Name != singlesName || len(singles.Tracks) != 2 {
		t.Fatalf("want both singles in one album, got %+v", singles)
	}
	if singles.Tracks[0].Path != "A/Singles/Kept.mp3" || singles.Tracks[1].Path != "A/Loose.mp3" {
		t.Fatalf("paths: %+v", singles.Tracks)
	}
}
