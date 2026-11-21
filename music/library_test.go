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

func TestSongOf(t *testing.T) {
	for _, c := range []struct{ title, channel, name, artist string }{
		{"Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)", "Rick Astley", "Never Gonna Give You Up", "Rick Astley"},
		{"Survivor - Eye Of The Tiger (Official HD Video)", "Survivor Band", "Eye Of The Tiger", "Survivor"},
		{"Dead Or Alive - You Spin Me Round (Like a Record) (Official Video)", "deadoralivetv", "You Spin Me Round (Like a Record)", "Dead Or Alive"},
		{"Here Comes The Sun - Remastered 2019", "The Beatles - Topic", "Here Comes The Sun - Remastered 2019", "The Beatles"},
		{"Bad Romance (Official Music Video)", "LadyGagaVEVO", "Bad Romance", "LadyGaga"},
		{"Wet- Dazey and The Scouts", "Brennan Wedl", "Wet", "Dazey and The Scouts"},
		{"Panic! At The Disco - House of Memories | Lyrics", "", "House of Memories", "Panic! At The Disco"},
		{`Plain White T's - "Hey There Delilah" (Official Music Video)`, "", "Hey There Delilah", "Plain White T's"},
		{"One Way Or Another", "BlondieMusicOfficial", "One Way Or Another", "Blondie"},
		{"The All-American Rejects - Dirty Little Secret (Official Music Video)", "", "Dirty Little Secret", "The All-American Rejects"},
		{"Paramore: Decode [OFFICIAL VIDEO]", "Paramore", "Decode", "Paramore"},
		{"Star Wars: Main Title", "John Williams", "Star Wars: Main Title", "John Williams"},
		{"Mitski – Washing Machine Heart [Official Music Video] [4K Remaster]", "", "Washing Machine Heart", "Mitski"},
	} {
		if name, artist := songOf(c.title, c.channel, map[string]bool{"dazey and the scouts": true}); name != c.name || artist != c.artist {
			t.Errorf("songOf(%q, %q) = %q, %q; want %q, %q", c.title, c.channel, name, artist, c.name, c.artist)
		}
	}
}
