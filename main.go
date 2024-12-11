package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type sound struct {
	name string
	url  string
}

type readCloserWrapper struct {
	io.Reader
	closer io.Closer
}

func (rc *readCloserWrapper) Close() error {
	return rc.closer.Close()
}

var OLD_FAITHFUL = sound{
	// https://www.nps.gov/yell/learn/photosmultimedia/sounds-oldfaithful.htm
	name: "Old Faithful (Remixed)",
	url:  "https://www.nps.gov/av/imr/avElement/yell-00150325YellowstoneOldFaithfulGeyserEruption3Mix3Alt101.mp3",
}
var BLACK_CANYON_TRAIL = sound{
	// https://www.nps.gov/romo/learn/photosmultimedia/sounds-ambient-soundscapes.htm
	name: "Stream Soundscape from the Black Canyon Trail",
	url:  "https://www.nps.gov/av/imr/avElement/romo-StreamAmbientROMO52516BlackCanyonTrailFinal1.mp3",
}
var LOWER_GEYSER_BASE = sound{
	// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
	name: "Soundscape - Lower Geyser Basin (Strong Wind)",
	url:  "https://www.nps.gov/av/imr/avElement/yell-040201LowerGeyserBasinWindInTreesBinaural01011.mp3",
}

func getApplicationDataDir() (string, error) {
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

func main() {
	dataDir, err := getApplicationDataDir()
	if err != nil {
		log.Fatal("Error getting XDG_DATA_HOME: ", err)
	}

	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		log.Fatal("Error creating XDG_DATA_HOME: ", err)
	}

	soundPath := filepath.Join(dataDir, filepath.Base(OLD_FAITHFUL.url))

	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		fmt.Println("Downloading sound:", LOWER_GEYSER_BASE.name)
		resp, err := http.Get(OLD_FAITHFUL.url)
		if err != nil {
			log.Fatalf("Failed to download file: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("HTTP error: %s", resp.Status)
		}

		file, err = os.Create(soundPath)
		if err != nil {
			log.Fatalf("Failed to create local file: %v", err)
		}

		io.Copy(file, resp.Body)
	} else if err != nil {
		log.Fatal("Error opening file: ", err)
	} else {
		defer file.Close()
	}

	stream, format, err := mp3.Decode(file)
	if err != nil {
		log.Fatal("Error decoding file: ", err)
	}
	defer stream.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Fatal("Error initializing speaker: ", err)
	}

	loopStream := beep.Loop(-1, stream)

	ctrl := &beep.Ctrl{Streamer: loopStream, Paused: false}
	speaker.Play(ctrl)

	if err := keyboard.Open(); err != nil {
		fmt.Printf("Error opening keyboard: %v\n", err)
		return
	}

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			log.Fatal("Error reading key: ", err)
		}

		if char == 'p' {
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
		} else if char == 'q' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			file.Close()
			stream.Close()
			keyboard.Close()
			break
		}
	}
}
