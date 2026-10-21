// Package speech speaks lines with Kokoro on this computer, natively, which
// is several times faster than the browser's WebAssembly copy of the same
// model. It needs cgo (sherpa-onnx); without it, Available reports why not.
//
// The model, about 330 MB, is downloaded from the sherpa-onnx releases on
// first use and kept in Dir.
package speech

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// Model is the Kokoro release the voices come from; it has the same voices
// as the browser's kokoro-js.
const Model = "kokoro-multi-lang-v1_0"

const modelURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/" + Model + ".tar.bz2"

// voices are the model's speaker ids, in order.
var voices = strings.Split("af_alloy,af_aoede,af_bella,af_heart,af_jessica,af_kore,af_nicole,af_nova,af_river,af_sarah,af_sky,am_adam,am_echo,am_eric,am_fenrir,am_liam,am_michael,am_onyx,am_puck,am_santa,bf_alice,bf_emma,bf_isabella,bf_lily,bm_daniel,bm_fable,bm_george,bm_lewis,ef_dora,em_alex,ff_siwis,hf_alpha,hf_beta,hm_omega,hm_psi,if_sara,im_nicola,jf_alpha,jf_gongitsune,jf_nezumi,jf_tebukuro,jm_kumo,pf_dora,pm_alex,pm_santa,zf_xiaobei,zf_xiaoni,zf_xiaoxiao,zf_xiaoyi,zm_yunjian,zm_yunxi,zm_yunxia,zm_yunyang,em_santa", ",")

// Voices speaks with Kokoro, a few lines at a time.
type Voices struct {
	// Dir holds the downloaded model.
	Dir string

	once    sync.Once
	slots   chan struct{}
	threads int

	mu      sync.Mutex
	have    bool       // the model is in Dir
	loading *fetching  // the download under way
	idle    []accented // loaded engines not speaking
	engines int        // engines loaded or loading, at most one per slot
}

// engine is one loaded copy of the model; it speaks one line at a time.
type engine interface {
	speak(text string, sid int, speed float32) ([]float32, int)
	close()
}

// accented is an engine and the accent it reads with.
type accented struct {
	engine
	lang string
}

type fetching struct {
	done chan struct{}
	err  error
}

func (v *Voices) init() {
	v.once.Do(func() {
		// Speaking is bound by memory bandwidth: on a 24-thread laptop, two
		// lines at a time on 8 threads each beat one on 24 or six on 4.
		cpus := runtime.NumCPU()
		workers := 2
		if cpus < 8 {
			workers = 1
		}
		v.threads = max(1, min(8, cpus/workers))
		v.slots = make(chan struct{}, workers)
	})
}

// Available says whether this binary can speak, and what using it involves.
func (v *Voices) Available() (bool, string) {
	if !native {
		return false, "built without cgo, so no voices on this computer"
	}
	if _, err := os.Stat(filepath.Join(v.Dir, Model, "model.onnx")); err != nil {
		return true, "on this computer (downloads the 330 MB voice model on first use)"
	}
	return true, "on this computer"
}

// Speak returns the line spoken by voice as 16-bit mono WAV.
func (v *Voices) Speak(ctx context.Context, text, voice string, speed float64) ([]byte, error) {
	if !native {
		return nil, errors.New("built without cgo, so no voices on this computer")
	}
	sid := slices.Index(voices, voice)
	if sid < 0 {
		return nil, fmt.Errorf("unknown voice %q", voice)
	}
	if speed <= 0 {
		speed = 1
	}
	v.init()
	if err := v.fetch(ctx); err != nil {
		return nil, err
	}
	select {
	case v.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-v.slots }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e, err := v.take(accent(voice))
	if err != nil {
		return nil, err
	}
	defer v.put(e)
	samples, rate := e.speak(strings.TrimSpace(text), sid, float32(speed))
	if len(samples) == 0 {
		return nil, errors.New("the voice model made no audio")
	}
	return wav(samples, rate), nil
}

// accent is the espeak voice a voice reads with: British voices start with
// b, and espeak's plain "en" is British.
func accent(voice string) string {
	if strings.HasPrefix(voice, "b") {
		return "en"
	}
	return "en-us"
}

// take returns an engine for an accent, loading one if none is free. The
// caller holds a slot, so when every slot already has an engine, one of
// them is free: an engine of the other accent, which makes way.
func (v *Voices) take(lang string) (accented, error) {
	v.mu.Lock()
	for i, e := range v.idle {
		if e.lang == lang {
			v.idle = slices.Delete(v.idle, i, i+1)
			v.mu.Unlock()
			return e, nil
		}
	}
	var drop engine
	if v.engines == cap(v.slots) {
		drop = v.idle[0].engine
		v.idle = slices.Delete(v.idle, 0, 1)
	} else {
		v.engines++
	}
	v.mu.Unlock()
	if drop != nil {
		drop.close()
	}
	e, err := load(filepath.Join(v.Dir, Model), lang, v.threads)
	if err != nil {
		v.mu.Lock()
		v.engines--
		v.mu.Unlock()
		return accented{}, err
	}
	return accented{e, lang}, nil
}

func (v *Voices) put(e accented) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.idle = append(v.idle, e)
}

// fetch downloads the model if it isn't in Dir yet. The download carries
// on if the request that started it goes away, so the next can use it; if
// it fails, the next request tries again.
func (v *Voices) fetch(ctx context.Context) error {
	v.mu.Lock()
	if v.have {
		v.mu.Unlock()
		return nil
	}
	dest := filepath.Join(v.Dir, Model)
	if _, err := os.Stat(filepath.Join(dest, "model.onnx")); err == nil {
		v.have = true
		v.mu.Unlock()
		return nil
	}
	f := v.loading
	if f == nil {
		f = &fetching{done: make(chan struct{})}
		v.loading = f
		go func() {
			slog.Info("downloading the voice model", "url", modelURL, "to", dest)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			f.err = download(ctx, v.Dir)
			if f.err != nil {
				slog.Error("downloading the voice model", "err", f.err)
			} else {
				slog.Info("voice model ready")
			}
			v.mu.Lock()
			v.have = f.err == nil
			v.loading = nil
			v.mu.Unlock()
			close(f.done)
		}()
	}
	v.mu.Unlock()
	select {
	case <-f.done:
		return f.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func download(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, ".download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading the voice model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading the voice model: %s", resp.Status)
	}
	tr := tar.NewReader(bzip2.NewReader(resp.Body))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("unpacking the voice model: %w", err)
		}
		name := filepath.Clean(h.Name)
		if !filepath.IsLocal(name) || !strings.HasPrefix(name, Model) {
			return fmt.Errorf("unpacking the voice model: unexpected file %q", h.Name)
		}
		path := filepath.Join(tmp, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return fmt.Errorf("unpacking the voice model: %w", err)
			}
		}
	}
	return os.Rename(filepath.Join(tmp, Model), filepath.Join(dir, Model))
}

// wav encodes samples as 16-bit mono PCM.
func wav(samples []float32, rate int) []byte {
	b := make([]byte, 44+2*len(samples))
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+2*len(samples)))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(2*rate))
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(2*len(samples)))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[44+2*i:], uint16(int16(math.Round(float64(max(-1, min(1, s)))*32767))))
	}
	return b
}
