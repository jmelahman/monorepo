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
	"github.com/gdamore/tcell/v2"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

var (
	version = "dev"

	sounds = []Sound{
		{
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-oldfaithful.htm
			name:   "Old Faithful (Remixed)",
			credit: "NPS/Jennifer Jerrett and Peter Comley",
			url:    "https://www.nps.gov/av/imr/avElement/yell-00150325YellowstoneOldFaithfulGeyserEruption3Mix3Alt101.mp3",
		},
		{
			// https://www.nps.gov/romo/learn/photosmultimedia/sounds-ambient-soundscapes.htm
			name:   "Stream Soundscape from the Black Canyon Trail",
			credit: "J. Job",
			url:    "https://www.nps.gov/av/imr/avElement/romo-StreamAmbientROMO52516BlackCanyonTrailFinal1.mp3",
		},
		{
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
			name:   "Soundscape - Lower Geyser Basin (Strong Wind)",
			credit: "NPS/Peter Comley",
			url:    "https://www.nps.gov/av/imr/avElement/yell-040201LowerGeyserBasinWindInTreesBinaural01011.mp3",
		},
	}
)

type Sound struct {
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
		fmt.Printf("\rDownloaded: %.0f MB", float64(pr.DownloadedSize)/(1024*1024))
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
		// We don't always get the Content-Length ahead of time and thus is life.
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

func ListPicker(items []Sound) (int, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return -1, err
	}
	defer screen.Fini()

	if err := screen.Init(); err != nil {
		return -1, err
	}

	style := tcell.StyleDefault
	selectedStyle := tcell.StyleDefault.Bold(true)

	selectedIndex := 0
	draw := func() {
		screen.Clear()
		for i, item := range items {
			styleToUse := style
			if i == selectedIndex {
				styleToUse = selectedStyle
			}

			line := fmt.Sprintf("%d) %s", i, item.name)
			for x, ch := range line {
				screen.SetContent(x, i, ch, nil, styleToUse)
			}
		}
		screen.Show()
	}

	draw()
	for {
		event := screen.PollEvent()
		switch ev := event.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape:
				return -1, fmt.Errorf("selection canceled")
			case tcell.KeyEnter:
				return selectedIndex, nil
			case tcell.KeyUp:
				if selectedIndex > 0 {
					selectedIndex--
					draw()
				}
			case tcell.KeyDown:
				if selectedIndex < len(items)-1 {
					selectedIndex++
					draw()
				}
			}
		}
	}
}

func playSound(dataDir string, sound Sound) (*beep.Ctrl, *os.File, beep.StreamSeekCloser, error) {
	soundPath := filepath.Join(dataDir, filepath.Base(sound.url))
	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		err := downloadFileWithProgress(sound.url, soundPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("Error downloading sound: %v", err)
		}
		file, err = os.Open(soundPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("Error opening file: %v", err)
		}
	} else if err != nil {
		return nil, nil, nil, fmt.Errorf("Error opening file: %v", err)
	}

	// TODO: Maybe log if this format differs from the BufferSize set globally.
	stream, _, err := mp3.Decode(file)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Error decoding file: %v", err)
	}

	loopStream, err := beep.Loop2(stream)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Error creating loop stream: %v", err)
	}

	ctrl := &beep.Ctrl{Streamer: loopStream, Paused: false}
	speaker.Play(ctrl)
	fmt.Printf("\r➤  \"%s\" by \"%s\"\n", sound.name, sound.credit)

	return ctrl, file, stream, nil
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

	sampleRate := beep.SampleRate(44100)
	err = speaker.Init(sampleRate, sampleRate.N(time.Second/10))
	if err != nil {
		log.Fatal("Error initializing speaker: ", err)
	}

	nowPlaying := sounds[0]
	ctrl, file, stream, err := playSound(dataDir, nowPlaying)
	doubleLine := true
	if err != nil {
		file.Close()
		stream.Close()
		log.Fatal("Error playing sound: ", err)
	}
	defer file.Close()
	defer stream.Close()

	if err := keyboard.Open(); err != nil {
		log.Fatal("Error opening keyboard: ", err)
	}
	defer keyboard.Close()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			log.Fatal("Error reading key: ", err)
		}

		switch char {
		case 'p': // Pause/Resume
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
			if ctrl.Paused {
				if doubleLine {
					fmt.Printf("\033[F")
				}
				fmt.Printf("\r⏸︎  \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
				doubleLine = true
			} else {
				if doubleLine {
					fmt.Printf("\033[F")
				}
				fmt.Printf("\r➤  \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
			}
		case 's': // Switch to the next sound
			file.Close()
			stream.Close()
			keyboard.Close()

			soundIndex, err := ListPicker(sounds)
			if err != nil {
				log.Fatal("Error selecting next sound: ", err)
			}
			nowPlaying = sounds[soundIndex]

			ctrl, file, stream, err = playSound(dataDir, nowPlaying)
			if err != nil {
				keyboard.Close()
				file.Close()
				log.Fatal("Error switching sound: ", err)
			}
			if err := keyboard.Open(); err != nil {
				log.Fatal("Error opening keyboard: ", err)
			}
			defer keyboard.Close()

		case '?': // Help
			fmt.Println("Available commands:")
			fmt.Println("\tp  pause/resume playback")
			fmt.Println("\tq  quit")
			fmt.Println("\ts  select new sound")
			doubleLine = false

		case 'q': // Quit
			return

		default:
			if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				return
			}
		}
	}
}
