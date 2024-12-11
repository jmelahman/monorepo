package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

type sound struct {
	name   string
	credit string
	url    string
}

type readCloserWrapper struct {
	io.Reader
	closer io.Closer
}

func (rc *readCloserWrapper) Close() error {
	return rc.closer.Close()
}

type ProgressReader struct {
	Reader         io.Reader
	TotalSize      int64
	DownloadedSize int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.DownloadedSize += int64(n)

	// TODO: Improve this UI.
	if pr.TotalSize > 0 {
		percent := float64(pr.DownloadedSize) / float64(pr.TotalSize) * 100
		fmt.Printf("\rProgress: %.2f%%", percent)
	} else {
		fmt.Printf("\rDownloaded: %d bytes", pr.DownloadedSize)
	}

	return n, err
}

func downloadFileWithProgress(url, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	contentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil || contentLength <= 0 {
		fmt.Println("Unable to determine file size; progress might not be accurate.")
	}

	progressReader := &ProgressReader{
		Reader:         resp.Body,
		TotalSize:      int64(contentLength),
		DownloadedSize: 0,
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

var (
	version = "dev"

	OLD_FAITHFUL = sound{
		// https://www.nps.gov/yell/learn/photosmultimedia/sounds-oldfaithful.htm
		name:   "Old Faithful (Remixed)",
		credit: "NPS/Jennifer Jerrett and Peter Comley",
		url:    "https://www.nps.gov/av/imr/avElement/yell-00150325YellowstoneOldFaithfulGeyserEruption3Mix3Alt101.mp3",
	}
	BLACK_CANYON_TRAIL = sound{
		// https://www.nps.gov/romo/learn/photosmultimedia/sounds-ambient-soundscapes.htm
		name:   "Stream Soundscape from the Black Canyon Trail",
		credit: "J. Job",
		url:    "https://www.nps.gov/av/imr/avElement/romo-StreamAmbientROMO52516BlackCanyonTrailFinal1.mp3",
	}
	LOWER_GEYSER_BASE = sound{
		// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
		name:   "Soundscape - Lower Geyser Basin (Strong Wind)",
		credit: "NPS/Peter Comley",
		url:    "https://www.nps.gov/av/imr/avElement/yell-040201LowerGeyserBasinWindInTreesBinaural01011.mp3",
	}
)

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
	fmt.Printf("Welcome to nature-sounds (%v). Press ? for a list of commands.\n", version)
	dataDir, err := getApplicationDataDir()
	if err != nil {
		log.Fatal("Error getting XDG_DATA_HOME: ", err)
	}

	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		log.Fatal("Error creating XDG_DATA_HOME: ", err)
	}

	nowPlaying := LOWER_GEYSER_BASE
	soundPath := filepath.Join(dataDir, filepath.Base(nowPlaying.url))

	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		err := downloadFileWithProgress(nowPlaying.url, soundPath)
		if err != nil {
			log.Fatal("Error downloading sound: ", err)
		}
		file, err = os.Open(soundPath)
		if err != nil {
			log.Fatal("Error opening file: ", err)
		}
	} else if err != nil {
		log.Fatal("Error opening file: ", err)
	}
	defer file.Close()

	stream, format, err := mp3.Decode(file)
	if err != nil {
		log.Fatal("Error decoding file: ", err)
	}
	defer stream.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Fatal("Error initializing speaker: ", err)
	}

	loopStream, err := beep.Loop2(stream)
	if err != nil {
		log.Fatal("Error creating loop stream: ", err)
	}

	ctrl := &beep.Ctrl{Streamer: loopStream, Paused: false}
	speaker.Play(ctrl)
	fmt.Printf("\r➤ \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)

	if err := keyboard.Open(); err != nil {
		fmt.Printf("Error opening keyboard: %v\n", err)
		return
	}
	defer keyboard.Close()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			log.Fatal("Error reading key: ", err)
		}
		if char == 'p' {
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
			if ctrl.Paused {
				fmt.Printf("\033[F\r⏸︎ \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
			} else {
				fmt.Printf("\033[F\r➤ \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
			}
		} else if char == '?' {
			// TODO: use tabwritter
			fmt.Println("\tp  pause/resume playback")
			fmt.Println("\tq  quit")
		} else if char == 'q' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			file.Close()
			stream.Close()
			keyboard.Close()
			break
		}
	}
}
