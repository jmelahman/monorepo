# kev-go

Runs [Kev](https://github.com/jaredpalmer/kev) decision models in pure Go, on the
CPU. Give it a state and typed questions and it returns calibrated probabilities.
It generates no text.

There's no cgo and no torch, so it builds with `CGO_ENABLED=0` and cross-compiles.
amd64 with AVX2 uses assembly kernels, and everything else uses slower generic ones.

## Setup

Download a checkpoint and its base model, then convert the checkpoint's head.
`head.pt` is a pickle, which kev-go won't open, so this step needs Python once, on
one machine. The result is an ordinary file you can copy around.

```sh
hf download jaredpalmer/kev-0.8b --local-dir kev-0.8b
hf download Qwen/Qwen3.5-0.8B-Base --local-dir qwen-base
uv run --with torch --with safetensors python -I tools/convert_head.py kev-0.8b
```

## Use

```go
model, err := kev.Open("kev-0.8b", kev.Options{Base: "qwen-base"})
defer model.Close()
```

`Model.Transport` plugs into the TypeSafe SDK, so existing System One code runs
locally with no other change:

```go
client, err := typesafe.New(
	typesafe.WithAPIKey("local"), // required by the SDK, ignored here
	typesafe.WithHTTPClient(&http.Client{Transport: model.Transport()}),
)
```

`Model.Handler` serves the same `POST /v1/systemone` API over HTTP, and `Model.Decide`
takes plain strings and returns raw probabilities.

## Cost

Kev-0.8B on a 12-core Ryzen AI 9 HX, with other programs running:

| | |
| --- | --- |
| Load | about 1.5 s |
| Memory | about 4 GB, plus about 43 KB per token of cached state |
| Speed | about 170 to 230 tokens/s, like torch on the same CPU |
| 2,576-token request | 11 to 15 s |
| Another question about a cached state | only the question's tokens |

Requests run one at a time, each using every worker. Kev validated this model up to
8,192 tokens of context, though like Kev's server this accepts up to 65,536.

## Fine-tunes

Everything about a model comes from its checkpoint, so a fine-tune of a supported
backbone loads the same way. Run `tools/convert_head.py` on it first. A checkpoint
with full weights rather than an adapter needs no `Base`.

Kev-0.8B is the only checkpoint tested. The Qwen3.5 architecture is implemented, so
4B and 9B should load, but they haven't been run. Kev-27B's mixture-of-experts base
isn't implemented.

## Tests

Tests compare against values in `testdata/`. Those come from `transformers` and `peft`
plus a transcription of Kev's encoding, not from running Kev's own server.

```sh
go test ./...                                   # what needs no weights
KEV_CHECKPOINT=kev-0.8b KEV_BASE=qwen-base go test ./...
```

## License

Apache-2.0, the same as Kev, which this ports parts of. See [NOTICE](NOTICE).
