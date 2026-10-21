// Package publish encodes an episode and files it where a podcast server,
// such as Audiobookshelf, picks it up: one folder per notebook.
package publish

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Episode is what is published.
type Episode struct {
	// Podcast names the folder the episode goes in: the notebook's title.
	Podcast     string
	Title       string
	Description string
	Date        time.Time
}

// Publisher writes episodes under Dir.
type Publisher struct {
	Dir string
	// FFmpeg is the ffmpeg binary (default "ffmpeg" on $PATH).
	FFmpeg string
}

func (p *Publisher) ffmpeg() string {
	if p.FFmpeg != "" {
		return p.FFmpeg
	}
	return "ffmpeg"
}

// Ready reports whether episodes can be published, and if not, why.
func (p *Publisher) Ready() error {
	if p.Dir == "" {
		return errors.New("no podcasts folder (start with --podcasts DIR)")
	}
	if st, err := os.Stat(p.Dir); err == nil && !st.IsDir() {
		return fmt.Errorf("%s isn't a folder", p.Dir)
	}
	if _, err := exec.LookPath(p.ffmpeg()); err != nil {
		return errors.New("ffmpeg isn't installed")
	}
	return nil
}

// Publish encodes audio, in any format ffmpeg reads, to a loudness
// normalized MP3 and returns the path it was written to.
func (p *Publisher) Publish(ctx context.Context, ep Episode, audio io.Reader) (string, error) {
	if err := p.Ready(); err != nil {
		return "", err
	}
	podcast := safeName(ep.Podcast)
	title := safeName(ep.Title)
	if podcast == "" || title == "" {
		return "", errors.New("an episode needs a podcast and a title")
	}
	if ep.Date.IsZero() {
		ep.Date = time.Now()
	}
	dir := filepath.Join(p.Dir, podcast)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := ep.Date.Format("2006-01-02") + " " + title
	// Encode beside the destination and rename, so the server never scans a
	// half-written file.
	tmp, err := os.CreateTemp(dir, ".publish-*.mp3")
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.CommandContext(ctx, p.ffmpeg(), "-hide_banner", "-loglevel", "error", "-y",
		"-i", "pipe:0",
		"-af", "loudnorm=I=-16:TP=-1.5:LRA=11",
		"-ac", "1", "-ar", "44100", "-codec:a", "libmp3lame", "-q:a", "3",
		"-id3v2_version", "3",
		"-metadata", "title="+ep.Title,
		"-metadata", "album="+ep.Podcast,
		"-metadata", "artist="+ep.Podcast,
		"-metadata", "genre=Podcast",
		"-metadata", "date="+ep.Date.Format("2006-01-02"),
		"-metadata", "comment="+ep.Description,
		"-f", "mp3", tmp.Name(),
	)
	cmd.Stdin = audio
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg: %s", cmp.Or(strings.TrimSpace(stderr.String()), err.Error()))
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return "", err
	}
	// A link, unlike a rename, fails if the name is taken, so two episodes
	// published at once can't both take it.
	for i := 1; ; i++ {
		dest := filepath.Join(dir, name+".mp3")
		if i > 1 {
			dest = filepath.Join(dir, fmt.Sprintf("%s (%d).mp3", name, i))
		}
		err := os.Link(tmp.Name(), dest)
		if err == nil {
			return dest, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
}

// safeName makes s usable as a file name everywhere the podcasts folder
// might be shared to.
func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, strings.ContainsRune(`/\:*?"<>|`, r):
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, ". ")
	if r := []rune(s); len(r) > 120 {
		s = strings.TrimSpace(string(r[:120]))
	}
	return s
}
