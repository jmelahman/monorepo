package player

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/jmelahman/nature-sounds/download"
	"github.com/jmelahman/nature-sounds/sounds"
)

type Player struct {
	ctrl   *beep.Ctrl
	file   *os.File
	stream beep.StreamSeekCloser
	volume *effects.Volume
}

func NewPlayer() *Player {
	return &Player{}
}

func (p *Player) PlaySound(dataDir string, sound sounds.Sound) error {
	soundPath := filepath.Join(dataDir, filepath.Base(sound.Url))
	file, err := os.Open(soundPath)
	if os.IsNotExist(err) {
		err := download.FileWithProgress(sound.Url, soundPath)
		if err != nil {
			os.Remove(soundPath)
			return fmt.Errorf("error downloading sound: %v", err)
		}
		file, err = os.Open(soundPath)
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}

	stream, _, err := mp3.Decode(file)
	if err != nil {
		os.Remove(file.Name())
		return fmt.Errorf("error decoding file: %v", err)
	}

	loopStream, err := beep.Loop2(stream)
	if err != nil {
		return fmt.Errorf("error creating loop stream: %v", err)
	}

	p.ctrl = &beep.Ctrl{Streamer: loopStream, Paused: false}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2, Volume: 0}
	p.file = file
	p.stream = stream

	speaker.Play(p.volume)
	fmt.Printf("\r➤  \"%s\" by \"%s\"\n", sound.Name, sound.Credit)
	return nil
}

func (p *Player) TogglePause() {
	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	speaker.Unlock()
}

func (p *Player) IsPaused() bool {
	return p.ctrl.Paused
}

func (p *Player) SetVolume(change float64) {
	speaker.Lock()
	p.volume.Volume += change
	speaker.Unlock()
	fmt.Printf("Volume: %.1f\r", p.volume.Volume)
}

func (p *Player) Close() {
	if p.file != nil {
		p.file.Close()
	}
	if p.stream != nil {
		p.stream.Close()
	}
}
