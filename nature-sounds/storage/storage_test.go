package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/nature-sounds/sounds"
)

func TestStorageOperations(t *testing.T) {
	tempDir := t.TempDir()

	testSound := sounds.Sound{
		Name:   "Test Sound",
		Credit: "Tester",
		Url:    "http://test.url",
	}

	// Test GetApplicationDataDir behavior
	dataDir, err := GetApplicationDataDir()
	if err != nil {
		t.Fatalf("GetApplicationDataDir failed: %v", err)
	}
	if dataDir == "" {
		t.Error("Expected non-empty data directory path")
	}

	// Test Save/Load
	err = SaveNowPlaying(tempDir, testSound.Url)
	if err != nil {
		t.Fatalf("SaveNowPlaying failed: %v", err)
	}

	// Should match our test sound
	sound := LoadLastPlayed(tempDir, []sounds.Sound{testSound})
	if sound.Name != testSound.Name {
		t.Errorf("Expected sound %v, got %v", testSound, sound)
	}

	// Test Remove
	RemoveNowPlaying(tempDir)
	_, err = os.Stat(filepath.Join(tempDir, "now_playing"))
	if !os.IsNotExist(err) {
		t.Error("RemoveNowPlaying failed - file still exists")
	}

	// Test Load with empty/no file
	emptySound := LoadLastPlayed(tempDir, []sounds.Sound{testSound})
	if emptySound.Name != "" {
		t.Errorf("Expected empty sound, got %v", emptySound)
	}
}
