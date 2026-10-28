# local-ai-detection — Claude Notes

A local detector for machine-generated text: llama.cpp embedded through yzma
(`llm/`), the zero-shot Binoculars detector on top of it (`binoculars/`), and
a CLI at the module root. The plan is to add an eval harness on public
datasets (RAID, MAGE, M4GT-Bench), a trained classifier of our own that uses
Binoculars' per-token scores among its features, and a TUI that highlights
tokens.

## Commands

- Tests: `go test ./...`. `binoculars.TestScore` runs the real models and
  skips unless they are already cached; `go run . <<< "some text"` fetches
  them (about 1.4 GB, into `~/.cache/local-ai-detection`).
- Run the CLI: `go run . [-tokens] [-processor cpu] [file]`.
- Lint: `prek run --all-files`. It is the same set CI runs; prefer it over
  invoking golangci-lint or govulncheck directly, since the hooks pin
  versions and provision the tools.

## Layout

- `llm/` — the reusable llama.cpp layer, kept free of anything detection
  specific so it can become its own module once a second project uses it.
  `Install`/`Init` handle the libraries (process-global, loaded once),
  `Spec`/`Fetch` pin and download GGUFs, `Model`/`Context` evaluate tokens.
  `Context.Logits` returns llama.cpp's memory, valid only until the next
  `Decode`.
- `binoculars/` — the detector. `Position` is the per-token math and is
  unit-tested with hand-computed logits; `Detector` drives the observer and
  performer contexts in lockstep, batch by batch, since both models' full
  logits at a position are needed together.
- `main.go` — the CLI. `main` does process plumbing only; everything testable
  lives in `run(ctx, args, stdin, stdout, stderr)`.
- `internal/version/` — ldflags-stamped version with a `runtime/debug`
  fallback.

## Conventions

- Model files and the llama.cpp release are pinned by digest: a `Spec`
  names a Hugging Face commit and the file's SHA-256, and `llm.LlamaVersion`
  carries yzma's pinned manifest digest. Changing a model means updating all
  of `Revision`, `SHA256` and `Size` (the Hub API's `tree/<rev>` lists `lfs.oid`).
- `binoculars.Threshold` is the paper's Falcon-7B value, not calibrated for
  Qwen3-0.6B; don't present verdicts as calibrated until the eval sets it.
- Tests go in `package <pkg>_test` and exercise the public API; table-driven
  with `t.Parallel()` on both the parent and the subtests.
- Struct fields: wide first, `bool` last (`aligo`).
- Sentinel errors are `ErrFoo` and matched with `errors.Is`.
