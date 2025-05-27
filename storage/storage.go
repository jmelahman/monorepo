package storage

import (
	"os"
	"path/filepath"
)

func GetApplicationDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, "nature-sounds"), nil
}

func SaveNowPlaying(dataDir string, soundURL string) error {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	return os.WriteFile(nowPlayingFile, []byte(soundURL), 0644)
}

func RemoveNowPlaying(dataDir string) {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	os.Remove(nowPlayingFile)
}

func LoadLastPlayed(dataDir string, availableSounds []Sound) Sound {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	data, err := os.ReadFile(nowPlayingFile)
	if err != nil {
		return Sound{}
	}
	lastURL := string(data)
	for _, sound := range availableSounds {
		if sound.Url == lastURL {
			return sound
		}
	}
	return Sound{}
}
