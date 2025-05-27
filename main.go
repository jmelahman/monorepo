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
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"

	"github.com/jmelahman/nature-sounds/download"
	"github.com/jmelahman/nature-sounds/sounds"
	"github.com/jmelahman/nature-sounds/storage"
)

var (
	version = "dev"
)

type ProgressReader struct {
	Reader         io.Reader
	TotalSize      int64
	DownloadedSize int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.DownloadedSize += int64(n)

	if pr.TotalSize > 0 {
		percent := float64(pr.DownloadedSize) / float64(pr.TotalSize) * 100
		fmt.Printf("\rDownloading.. %.0f%%", percent)
	} else {
		fmt.Printf("\rDownloading... %.0f MB", float64(pr.DownloadedSize)/(1024*1024))
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

	contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))

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


func ListPicker(items []sounds.Sound) (int, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return -1, err
	}
	defer screen.Fini()

	if err := screen.Init(); err != nil {
		return -1, err
	}

	style := tcell.StyleDefault
	selectedStyle := tcell.StyleDefault.Bold(true).Underline(true)

	selectedIndex := 0
	draw := func() {
		screen.Clear()
		for i, item := range items {
			styleToUse := style
			if i == selectedIndex {
				styleToUse = selectedStyle
			}

			line := fmt.Sprintf("%d) %s", i+1, item.Name)
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

func playSound(dataDir string, sound sounds.Sound) (*beep.Ctrl, *os.File, beep.StreamSeekCloser, *effects.Volume, error) {
	soundPath := filepath.Join(dataDir, filepath.Base(sound.Url))
	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		err := download.FileWithProgress(sound.Url, soundPath)
		if err != nil {
			os.Remove(soundPath)
			return nil, nil, nil, nil, fmt.Errorf("Error downloading sound: %v", err)
		}
		file, err = os.Open(soundPath)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("Error opening file: %v", err)
		}
	} else if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Error opening file: %v", err)
	}

	// TODO: Maybe log if this format differs from the BufferSize set globally.
	stream, _, err := mp3.Decode(file)
	if err != nil {
		// If there was an error decoding, the file is likely corrupt (possibly empty) and should be
		// removed.
		os.Remove(file.Name())
		return nil, nil, nil, nil, fmt.Errorf("Error decoding file: %v", err)
	}

	loopStream, err := beep.Loop2(stream)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Error creating loop stream: %v", err)
	}

	ctrl := &beep.Ctrl{Streamer: loopStream, Paused: false}
	volume := &effects.Volume{Streamer: ctrl, Base: 2, Volume: 0}
	speaker.Play(volume)
	fmt.Printf("\r➤  \"%s\" by \"%s\"\n", sound.Name, sound.Credit)

	return ctrl, file, stream, volume, nil
}

func main() {
	fmt.Printf("Welcome to nature-sounds (%v). Press ? for a list of commands.\n", version)
	dataDir, err := storage.GetApplicationDataDir()
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

	nowPlaying := storage.LoadLastPlayed(dataDir, sounds.Sounds)
	if nowPlaying.Name == "" {
		soundIndex, err := ListPicker(sounds.Sounds)
		if err != nil {
			log.Fatal("Error selecting next sound: ", err)
		}

		nowPlaying = sounds.Sounds[soundIndex]
	}

	ctrl, file, stream, volume, err := playSound(dataDir, nowPlaying)
	doubleLine := true
	if err != nil {
		fmt.Printf("Error playing sound \"%s\": %v\n", nowPlaying.Name, err)
		if file != nil {
			file.Close()
		}
		if stream != nil {
			stream.Close()
		}
		removeNowPlaying(dataDir)
		os.Exit(1)
	}
	defer file.Close()
	defer stream.Close()

	// TODO: Warn on error.
	_ = storage.SaveNowPlaying(dataDir, nowPlaying.Url)

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
			keyboard.Close()

			soundIndex, err := ListPicker(sounds.Sounds)
			if err != nil {
				log.Fatal("Error selecting next sound: ", err)
			}

			file.Close()
			stream.Close()
			ctrl, file, stream, volume, err = playSound(dataDir, sounds.Sounds[soundIndex])
			doubleLine = true
			if err != nil {
				fmt.Printf("Error switching to sound \"%s\": %v\n", sounds.Sounds[soundIndex].Name, err)
				if file != nil {
					file.Close()
				}
				if stream != nil {
					stream.Close()
				}

				ctrl, file, stream, volume, err = playSound(dataDir, nowPlaying)
				if err != nil {
					removeNowPlaying(dataDir)
					log.Fatal("Error playing previous sound: ", err)
				}
			} else {
				nowPlaying = sounds.Sounds[soundIndex]
			}

			// TODO: Warn on error.
			_ = saveNowPlaying(dataDir, nowPlaying)

			if err := keyboard.Open(); err != nil {
				log.Fatal("Error opening keyboard: ", err)
			}
			defer file.Close()
			defer stream.Close()
			defer keyboard.Close()

		case '?': // Help
			fmt.Println("Available commands:")
			fmt.Println("\tp  pause/resume playback")
			fmt.Println("\tq  quit")
			fmt.Println("\ts  select new sound")
			fmt.Println("\t(  volume down")
			fmt.Println("\t)  volume up")
			doubleLine = false

		case ')': // Volume up
			speaker.Lock()
			volume.Volume += 0.1
			speaker.Unlock()
			fmt.Printf("Volume: %.1f\r", volume.Volume)

		case '(': // Volume down
			speaker.Lock()
			volume.Volume -= 0.1
			speaker.Unlock()
			fmt.Printf("Volume: %.1f\r", volume.Volume)

		default:
			if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				return
			}
		}
	}
}
