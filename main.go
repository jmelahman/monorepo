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
)

var (
	version = "dev"

	// TODO: Maybe add some from https://www.nps.gov/maps/stories/soundscapes-of-the-seashore.html
	sounds = []Sound{
		{
			name:   "Dawn Soundscape from Big Meadows",
			credit: "J. Job",
			url:    "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/romo-DawnAmbientROMO6816BigMeadowsFinal1.mp3",
		},
		{
			name:   "Dawn Soundscape from the Sun Valley Trial",
			credit: "J. Job",
			url:    "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/romo-DawnAmbientROMO6916SunValleyTrailFinal1.mp3",
		},
		{
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
			name:   "Lake",
			credit: "NPS & MSU Acoustic Atlas/Jennifer Jerrett",
			url:    "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-YELLLakeSoundscape20160914T03ms.mp3",
		},
		{
			name:   "Lower Geyser Basin (Strong Wind)",
			credit: "NPS/Peter Comley",
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-040201LowerGeyserBasinWindInTreesBinaural01011.mp3",
		},
		{
			name:   "Old Faithful (Remixed)",
			credit: "NPS/Jennifer Jerrett and Peter Comley",
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-oldfaithful.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-00150325YellowstoneOldFaithfulGeyserEruption3Mix3Alt101.mp3",
		},
		{
			name:   "Stream Soundscape from the Black Canyon Trail",
			credit: "J. Job",
			// https://www.nps.gov/romo/learn/photosmultimedia/sounds-ambient-soundscapes.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/romo-StreamAmbientROMO52516BlackCanyonTrailFinal1.mp3",
		},
		{
			name:   "Thunder (and American Robin)",
			credit: "NPS/Jennifer Jerrett",
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-thunder.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-Thunderandbirds140704.mp3",
		},
		{
			name:   "Thunderstorm Soundscape from the Black Canyon Trail",
			credit: "J. Job",
			url:    "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/romo-ThunderstormAmbientROMO52616BlackCanyonTrail1.mp3",
		},
		{
			name:   "Wind on Peale Island",
			credit: "NPS & MSU Acoustic Atlas/Jennifer Jerrett",
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-YELLCabinSoundsWind20160912T032.mp3",
		},
		{
			name:   "Wind Soundscape from Gem Lake",
			credit: "J. Job",
			url:    "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/romo-WindAmbientGemLakeROMO52516Final1.mp3",
		},
		{
			name:   "Woodstove",
			credit: "NPS & MSU Acoustic Atlas",
			// https://www.nps.gov/yell/learn/photosmultimedia/sounds-soundscapes.htm
			url: "https://www.nps.gov/nps-audiovideo/legacy/mp3/imr/avElement/yell-YELLPealeCabinWoodstove20160914T15ms.mp3",
		},
	}
)

type Sound struct {
	name   string
	credit string
	url    string
}

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

func saveNowPlaying(dataDir string, sound Sound) error {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	return os.WriteFile(nowPlayingFile, []byte(sound.url), 0644)
}

func removeNowPlaying(dataDir string) {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	os.Remove(nowPlayingFile)
}

func loadLastPlayed(dataDir string) Sound {
	nowPlayingFile := filepath.Join(dataDir, "now_playing")
	data, err := os.ReadFile(nowPlayingFile)
	if err != nil {
		return Sound{}
	}
	lastURL := string(data)
	for _, sound := range sounds {
		if sound.url == lastURL {
			return sound
		}
	}
	return Sound{}
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
	selectedStyle := tcell.StyleDefault.Bold(true).Underline(true)

	selectedIndex := 0
	draw := func() {
		screen.Clear()
		for i, item := range items {
			styleToUse := style
			if i == selectedIndex {
				styleToUse = selectedStyle
			}

			line := fmt.Sprintf("%d) %s", i+1, item.name)
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

func playSound(dataDir string, sound Sound) (*beep.Ctrl, *os.File, beep.StreamSeekCloser, *effects.Volume, error) {
	soundPath := filepath.Join(dataDir, filepath.Base(sound.url))
	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		err := downloadFileWithProgress(sound.url, soundPath)
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
	fmt.Printf("\r➤  \"%s\" by \"%s\"\n", sound.name, sound.credit)

	return ctrl, file, stream, volume, nil
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

	nowPlaying := loadLastPlayed(dataDir)
	if nowPlaying.name == "" {
		soundIndex, err := ListPicker(sounds)
		if err != nil {
			log.Fatal("Error selecting next sound: ", err)
		}

		nowPlaying = sounds[soundIndex]
	}

	ctrl, file, stream, volume, err := playSound(dataDir, nowPlaying)
	doubleLine := true
	if err != nil {
		fmt.Printf("Error playing sound \"%s\": %v\n", nowPlaying.name, err)
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
	_ = saveNowPlaying(dataDir, nowPlaying)

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
				fmt.Printf("\r❚❚ \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
				doubleLine = true
			} else {
				if doubleLine {
					fmt.Printf("\033[F")
				}
				fmt.Printf("\r➤  \"%s\" by \"%s\"\n", nowPlaying.name, nowPlaying.credit)
			}
		case 'q': // Quit
			return
		case 's': // Switch to the next sound
			keyboard.Close()

			soundIndex, err := ListPicker(sounds)
			if err != nil {
				log.Fatal("Error selecting next sound: ", err)
			}

			file.Close()
			stream.Close()
			ctrl, file, stream, volume, err = playSound(dataDir, sounds[soundIndex])
			doubleLine = true
			if err != nil {
				fmt.Printf("Error switching to sound \"%s\": %v\n", sounds[soundIndex].name, err)
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
				nowPlaying = sounds[soundIndex]
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
