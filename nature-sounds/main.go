package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/jmelahman/nature-sounds/download"
	"github.com/jmelahman/nature-sounds/engine"
	"github.com/jmelahman/nature-sounds/picker"
	"github.com/jmelahman/nature-sounds/sounds"
	"github.com/jmelahman/nature-sounds/storage"
	"github.com/spf13/cobra"
)

var (
	version     = "dev"
	downloadAll bool
)

var rootCmd = &cobra.Command{
	Use:   "nature-sounds",
	Short: "A nature sounds player",
	Long:  "A command-line nature sounds player with interactive controls",
	Run:   runNatureSounds,
}

func init() {
	rootCmd.Flags().BoolVar(&downloadAll, "download-all", false, "Download all sounds before starting")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runNatureSounds(cmd *cobra.Command, args []string) {
	fmt.Printf("Welcome to nature-sounds (%v). Press ? for a list of commands.\n", version)
	dataDir, err := storage.GetApplicationDataDir()
	if err != nil || dataDir == "" {
		log.Fatal("Error getting application data directory: ", err)
	}

	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		log.Fatal("Error creating application data directory: ", err)
	}

	// Download all sounds if flag is set
	if downloadAll {
		fmt.Println("Downloading all sounds...")
		for i, sound := range sounds.Sounds {
			soundPath := filepath.Join(dataDir, filepath.Base(sound.Url))
			if _, err := os.Stat(soundPath); os.IsNotExist(err) {
				fmt.Printf("Downloading %d/%d: %s\n", i+1, len(sounds.Sounds), sound.Name)
				if err := download.FileWithProgress(sound.Url, soundPath); err != nil {
					fmt.Printf("Error downloading %s: %v\n", sound.Name, err)
					if err := os.Remove(soundPath); err != nil {
						fmt.Printf("Error removing incomplete file: %v\n", err)
					}
				} else {
					fmt.Println() // New line after progress
				}
			} else {
				fmt.Printf("Already downloaded %d/%d: %s\n", i+1, len(sounds.Sounds), sound.Name)
			}
		}
		fmt.Println("Download complete!")
	}

	nowPlaying := storage.LoadLastPlayed(dataDir, sounds.Sounds)
	if nowPlaying.Name == "" {
		soundIndex, err := picker.ListPicker(sounds.Sounds)
		if err != nil {
			log.Fatal("Error selecting next sound: ", err)
		}

		nowPlaying = sounds.Sounds[soundIndex]
	}

	mixer := engine.NewMixer(engine.SampleRate)
	if err := engine.Start(mixer); err != nil {
		log.Fatal("Error initializing speaker: ", err)
	}
	defer engine.Stop(mixer)

	layer, err := play(mixer, dataDir, nowPlaying, 0)
	if err != nil {
		fmt.Printf("Error playing sound \"%s\": %v\n", nowPlaying.Name, err)
		storage.RemoveNowPlaying(dataDir)
		engine.Stop(mixer)
		os.Exit(1)
	}
	saveNowPlaying(dataDir, nowPlaying)
	doubleLine := true

	if err := keyboard.Open(); err != nil {
		log.Fatal("Error opening keyboard: ", err)
	}
	// keyboard.Close is idempotent, so this covers every return even though
	// switching sounds closes and reopens the keyboard around the picker.
	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Printf("Error closing keyboard: %v", err)
		}
	}()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			log.Fatal("Error reading key: ", err)
		}

		switch char {
		case 'p': // Pause/Resume
			mixer.SetPaused(!mixer.Paused())
			if doubleLine {
				fmt.Printf("\033[F")
			}
			if mixer.Paused() {
				fmt.Printf("\r❚❚ \"%s\" by \"%s\"\n", nowPlaying.Name, nowPlaying.Credit)
			} else {
				fmt.Printf("\r➤  \"%s\" by \"%s\"\n", nowPlaying.Name, nowPlaying.Credit)
			}
			doubleLine = true
		case 'q': // Quit
			return
		case 's': // Switch to the next sound
			if err := keyboard.Close(); err != nil {
				log.Printf("Error closing keyboard: %v", err)
			}

			soundIndex, err := picker.ListPicker(sounds.Sounds)
			if err != nil {
				log.Fatal("Error selecting next sound: ", err)
			}

			// The new sound fades in while the old one fades out, so a
			// failure to open the new one leaves the old one playing.
			next := sounds.Sounds[soundIndex]
			nextLayer, err := play(mixer, dataDir, next, crossfade)
			doubleLine = true
			if err != nil {
				fmt.Printf("Error switching to sound \"%s\": %v\n", next.Name, err)
			} else {
				layer.Remove(crossfade)
				layer, nowPlaying = nextLayer, next
				mixer.SetPaused(false)
				saveNowPlaying(dataDir, nowPlaying)
			}

			if err := keyboard.Open(); err != nil {
				log.Fatal("Error opening keyboard: ", err)
			}

		case '?': // Help
			fmt.Println("Available commands:")
			fmt.Println("\tp  pause/resume playback")
			fmt.Println("\tq  quit")
			fmt.Println("\ts  select new sound")
			fmt.Println("\t(  volume down")
			fmt.Println("\t)  volume up")
			doubleLine = false

		case ')': // Volume up
			fmt.Printf("Volume: %+.0f dB  \r", mixer.SetVolume(mixer.Volume()+volumeStep))

		case '(': // Volume down
			fmt.Printf("Volume: %+.0f dB  \r", mixer.SetVolume(mixer.Volume()-volumeStep))

		default:
			if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				return
			}
		}
	}
}

const (
	// crossfade is how long switching sounds takes.
	crossfade = 2 * time.Second
	// volumeStep is one press of ( or ). 2 dB is about the smallest change
	// that is clearly audible on steady sound.
	volumeStep = 2
)

// play opens sound, downloading it first if needed, and adds it to the mixer.
func play(mixer *engine.Mixer, dataDir string, sound sounds.Sound, fade time.Duration) (*engine.Layer, error) {
	path, err := fetch(dataDir, sound)
	if err != nil {
		return nil, err
	}
	rec, err := engine.OpenRecording(path, mixer.SampleRate())
	if errors.Is(err, engine.ErrDecode) {
		// Most likely a download that was cut short; fetch it again next time.
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("error removing file: %w", err)
		}
	}
	if err != nil {
		return nil, err
	}
	fmt.Printf("\r➤  \"%s\" by \"%s\"\n", sound.Name, sound.Credit)
	return mixer.Add(rec, fade), nil
}

// fetch returns the local path of sound, downloading it if it is missing.
func fetch(dataDir string, sound sounds.Sound) (string, error) {
	path := filepath.Join(dataDir, filepath.Base(sound.Url))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, err
	}
	if err := download.FileWithProgress(sound.Url, path); err != nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("error removing sound file: %w", err)
		}
		return "", fmt.Errorf("error downloading sound: %w", err)
	}
	return path, nil
}

// saveNowPlaying remembers sound for the next launch. Failing to is not worth
// stopping playback over, so it only warns.
func saveNowPlaying(dataDir string, sound sounds.Sound) {
	if err := storage.SaveNowPlaying(dataDir, sound.Url); err != nil {
		fmt.Printf("Warning: could not remember \"%s\" for next time: %v\n", sound.Name, err)
	}
}
