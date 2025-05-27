package main

import (
	"fmt"
	"log"
	"os"

	"github.com/eiannone/keyboard"
	"github.com/jmelahman/nature-sounds/picker"
	"github.com/jmelahman/nature-sounds/player"
	"github.com/jmelahman/nature-sounds/sounds"
	"github.com/jmelahman/nature-sounds/storage"
)

var (
	version = "dev"
)

func main() {
	fmt.Printf("Welcome to nature-sounds (%v). Press ? for a list of commands.\n", version)
	dataDir, err := storage.GetApplicationDataDir()
	if err != nil || dataDir == "" {
		log.Fatal("Error getting application data directory: ", err)
	}

	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		log.Fatal("Error creating application data directory: ", err)
	}

	nowPlaying := storage.LoadLastPlayed(dataDir, sounds.Sounds)
	if nowPlaying.Name == "" {
		soundIndex, err := picker.ListPicker(sounds.Sounds)
		if err != nil {
			log.Fatal("Error selecting next sound: ", err)
		}

		nowPlaying = sounds.Sounds[soundIndex]
	}

	player := player.NewPlayer()

	err = player.Init()
	if err != nil {
		log.Fatal("Error initializing speaker: ", err)
	}

	err = player.PlaySound(dataDir, nowPlaying)
	doubleLine := true
	if err != nil {
		fmt.Printf("Error playing sound \"%s\": %v\n", nowPlaying.Name, err)
		player.Close()
		storage.RemoveNowPlaying(dataDir)
		os.Exit(1)
	}
	defer player.Close()

	// TODO: Warn on error.
	_ = storage.SaveNowPlaying(dataDir, nowPlaying.Url)

	if err := keyboard.Open(); err != nil {
		log.Fatal("Error opening keyboard: ", err)
	}
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
			player.TogglePause()
			if player.IsPaused() {
				if doubleLine {
					fmt.Printf("\033[F")
				}
				fmt.Printf("\r❚❚ \"%s\" by \"%s\"\n", nowPlaying.Name, nowPlaying.Credit)
				doubleLine = true
			} else {
				if doubleLine {
					fmt.Printf("\033[F")
				}
				fmt.Printf("\r➤  \"%s\" by \"%s\"\n", nowPlaying.Name, nowPlaying.Credit)
			}
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

			player.Close()
			err = player.PlaySound(dataDir, sounds.Sounds[soundIndex])
			doubleLine = true
			if err != nil {
				fmt.Printf("Error switching to sound \"%s\": %v\n", sounds.Sounds[soundIndex].Name, err)
				player.Close()
				err = player.PlaySound(dataDir, nowPlaying)
				if err != nil {
					storage.RemoveNowPlaying(dataDir)
					log.Fatal("Error playing previous sound: ", err)
				}
			} else {
				nowPlaying = sounds.Sounds[soundIndex]
			}

			// TODO: Warn on error.
			_ = storage.SaveNowPlaying(dataDir, nowPlaying.Url)

			if err := keyboard.Open(); err != nil {
				log.Fatal("Error opening keyboard: ", err)
			}
			defer player.Close()
			defer func() {
				if err := keyboard.Close(); err != nil {
					log.Printf("Error closing keyboard: %v", err)
				}
			}()

		case '?': // Help
			fmt.Println("Available commands:")
			fmt.Println("\tp  pause/resume playback")
			fmt.Println("\tq  quit")
			fmt.Println("\ts  select new sound")
			fmt.Println("\t(  volume down")
			fmt.Println("\t)  volume up")
			doubleLine = false

		case ')': // Volume up
			player.SetVolume(0.1)

		case '(': // Volume down
			player.SetVolume(-0.1)

		default:
			if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				return
			}
		}
	}
}
