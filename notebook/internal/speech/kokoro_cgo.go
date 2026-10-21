//go:build cgo

package speech

import (
	"errors"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const native = true

type kokoro struct{ tts *sherpa.OfflineTts }

func load(dir, lang string, threads int) (engine, error) {
	lexicon := "lexicon-us-en.txt"
	if lang == "en" {
		lexicon = "lexicon-gb-en.txt"
	}
	cfg := sherpa.OfflineTtsConfig{
		Model: sherpa.OfflineTtsModelConfig{
			Kokoro: sherpa.OfflineTtsKokoroModelConfig{
				Model:       filepath.Join(dir, "model.onnx"),
				Voices:      filepath.Join(dir, "voices.bin"),
				Tokens:      filepath.Join(dir, "tokens.txt"),
				DataDir:     filepath.Join(dir, "espeak-ng-data"),
				Lexicon:     filepath.Join(dir, lexicon),
				Lang:        lang,
				LengthScale: 1,
			},
			NumThreads: threads,
			Provider:   "cpu",
		},
		MaxNumSentences: 1,
	}
	tts := sherpa.NewOfflineTts(&cfg)
	if tts == nil {
		return nil, errors.New("the voice model in " + dir + " didn't load; delete that folder to download it again")
	}
	return kokoro{tts}, nil
}

func (k kokoro) speak(text string, sid int, speed float32) ([]float32, int) {
	audio := k.tts.Generate(text, sid, speed)
	if audio == nil {
		return nil, 0
	}
	return audio.Samples, audio.SampleRate
}

func (k kokoro) close() { sherpa.DeleteOfflineTts(k.tts) }
