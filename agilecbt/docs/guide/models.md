# Choosing a model

The coach talks with people about low mood, anxiety and sometimes crisis, and
it keeps your plan up to date by calling tools. Both depend heavily on the
model. A model that can chat pleasantly can still skip the crisis line or say
"I moved it to Today" without moving anything. These results come from the
[coach benchmarks](/guide/benchmarks), which play scripted conversations
through the real coach and score every reply.

## Summary

| If you… | Use |
| --- | --- |
| can run a 27–31B model | `qwen3.8:27b` (the default) or `gemma4:31b` |
| can't, and are fine with a hosted model | `openai/gpt-5.6-luna` or `google/gemini-3.8-flash` via [OpenRouter](/guide/ai#openrouter) |
| can only run a small model | Not supported yet: none tested was safe or reliable enough |

Whatever you pick, set [`crisis_resources`](/guide/configuration#crisis-resources)
for where you live. No model is guaranteed to share them every time.

## Recommendations

- **`qwen3.8:27b` and `gemma4:31b` are the best local models.** `qwen3.8:27b`
  had 100% tool recall and its one safety miss was an empty reply.
  `gemma4:31b` passed the most scenarios, but gave no emergency number in two
  crisis scenarios.
- **Skip `qwen3.6:35b-a3b`.** It is fast but made a third of the needed tool
  calls.
- **Avoid models under 27B.** They skip crisis resources and claim changes they
  didn't make. `gemma4:12b` is the closest, at 57% tool recall.
- **Hosted models trade privacy for quality.** `gpt-5.6-luna` passed every
  safety scenario and is cheap. Your check-ins leave your machine.
- **Price doesn't buy safety.** `claude-sonnet-5` gave no emergency number to
  someone hiding from a violent partner. Test before switching.

## Results

All models got the same prompt and tools. Models are sorted by scenarios
passed, but check crisis safety and tool recall too: a model that passes more
scenarios can still skip tool calls or crisis resources. Open **Table** under
each chart for phantom actions, unwanted writes and pass rates by area.

### Models you can run yourself

Up to about 35B parameters, small enough for one GPU or a well-equipped
Mac. These ran on Ollama on one machine.

![Scenarios passed, crisis safety and tool recall for models up to about 35B](/benchmarks/models-small.svg)

::: details Table
<!--@include: ./_model-results-small.md-->
:::

### Large hosted models

These ran through OpenRouter.

![Scenarios passed, crisis safety and tool recall for large hosted models](/benchmarks/models-large.svg)

::: details Table
<!--@include: ./_model-results-large.md-->
:::

<a href="/AgileCBT/benchmarks/report.html" target="_blank">Open the full report</a>
to see every transcript, tool call and failed check, with a scenario-by-model
grid.

- **Safety**: crisis scenarios (explicit, passive or joking ideation,
  self-harm, harm to others, immediate danger) where the reply shared crisis
  resources. These must pass every run.
- **Scenarios**: all scenarios that met their pass threshold, including style
  rules such as one question per reply and no em dashes.
- **Tool recall**: turns that needed a tool call and got every required call.
- **Phantom actions**: replies claiming a change that nothing saved.
- **Unwanted writes**: turns where the model changed data it should have asked
  about first.

Reply times aren't listed, since local ones depend on your hardware; the full
report has them.

## Caveats

- One run per scenario is a smoke test. Models sample differently each time,
  so treat a single pass or fail as a hint, not a verdict.
- The scenarios are synthetic and in English, and use the default prompt and
  tool set.
- Results change with the Ollama version, quantization, the hosting provider
  and settings such as `reasoning_effort`. The same `qwen3.8:27b` weights
  passed 7 of 8 safety scenarios locally and 4 of 8 on OpenRouter, mostly by
  asking more than one question per reply. The full report includes the
  hosted copies of `qwen3.8`, `gemma-4` and `mistral-small`.
- `openai/gpt-oss-120b`, `z-ai/glm-5.3` and `google/gemini-3.8-flash` reject
  `reasoning_effort = "none"`, so they ran with `low`. Set the same if you use
  them.

## Testing your own model

From a checkout of the repo (the scenarios live in `evals/scenarios/`):

```sh
ollama pull <model>
agilecbt eval --model <model> --runs 3 --no-baseline
```

For a hosted model, list it in a matrix file with its `base_url` and
`api_key_env` (see `evals/models.hosted.toml`) and run it with
`--matrix --matrix-file <file>`.

It prints a summary and saves an HTML report under `evals/results/`. See
[Coach benchmarks](/guide/benchmarks) for reading the report and adding
scenarios.
