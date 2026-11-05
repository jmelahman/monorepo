# local-ai-detection

[![Test status](https://github.com/jmelahman/local-ai-detection/actions/workflows/pre-commit.yml/badge.svg)](https://github.com/jmelahman/local-ai-detection/actions/workflows/pre-commit.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/local-ai-detection.svg)](https://pkg.go.dev/github.com/jmelahman/local-ai-detection)

Detects machine-generated text entirely on your machine. Nothing is sent
anywhere: llama.cpp and the models it runs are downloaded on first use,
pinned by SHA-256, and cached.

```console
$ local-ai-detection essay.txt
machine (score 0.8701, threshold 0.9015, uncalibrated; log-ppl 1.409, log-x-ppl 1.620)
$ pbpaste | local-ai-detection --tokens
```

## How it works

Today it is a zero-shot detector, [Binoculars](https://arxiv.org/abs/2401.12070):
a language model finds machine-generated text unsurprising, and Binoculars
divides that surprise by how surprising the text's context makes *any*
continuation, so a predictable subject alone does not read as machine
written. Scores well below 1 are machine-like. `--tokens` shows both numbers
for every token.

The models are Qwen3-0.6B, base and instruction-tuned (about 1.4 GB
together), run through [yzma](https://github.com/hybridgroup/yzma), which
loads llama.cpp's prebuilt libraries without cgo.

The threshold is still the paper's, for a different model pair; the
`cmd/eval` harness measures the detector against MAGE so that calibrating it,
and training a model of our own on top of these scores, can be done against
numbers rather than guesses. See [`eval/README.md`](eval/README.md).

## How well it works

On a 1,000-text sample of MAGE, the detector's ROC-AUC is **0.60** (95% CI
0.56–0.63): it separates text from large models well (AUC 0.95 for GPT-3
7B–65B and GLM130B, 0.87 for text-davinci), but is at or below chance on many
smaller generators, and weakest when a model continues a human prompt. The
shipped threshold also flags 11.7% of human text. The full numbers, method and
caveats are in [`eval/README.md`](eval/README.md).

## Flags

| Flag | Default | |
| --- | --- | --- |
| `--cache` | `$XDG_CACHE_HOME/local-ai-detection` | where llama.cpp and the models are kept |
| `--processor` | `cpu` | llama.cpp build: `cpu`, `cuda`, `vulkan`, `metal`, `rocm` |
| `--threshold` | `0.9015` | scores below it are called machine-generated |
| `--tokens` | off | print every token's surprise and expected surprise |

## Packages

- [`llm`](llm) — embeds llama.cpp for scoring: installs the libraries,
  fetches pinned GGUF models, and exposes per-position logits.
- [`binoculars`](binoculars) — the Binoculars detector, with per-token
  results.

## Development

```sh
go test ./...                 # model tests skip until the CLI has fetched the models
prek run --all-files          # the checks CI runs
```

To benchmark the detector on labelled text, see [`eval/README.md`](eval/README.md).
