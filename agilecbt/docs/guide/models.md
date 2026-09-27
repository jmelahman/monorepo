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
| can run a 26–31B model | `gemma4:26b` (the default) or `gemma4:31b` |
| can run a 12B model | `gemma4:12b`, but check its crisis replies |
| are fine with a hosted model | `openai/gpt-5.6-luna` or `google/gemini-3.8-flash` via [OpenRouter](/guide/ai#openrouter) |
| can only run a model under 12B | Not supported yet: none tested was safe or reliable enough |

Whatever you pick, set [`crisis_resources`](/guide/configuration#crisis-resources)
for where you live.

::: info Results predate the crisis classifier
These results were recorded with the coach's prompt alone. The app now runs a
[crisis classifier](/guide/safety) before each reply, which turns tools off and
makes sure the crisis lines reach the person when it flags a message. Its
phrase list catches explicit messages by default, and adding a
[`safety_model`](/guide/safety#adding-a-classifier-model) targets most of the
safety misses below, but the models haven't been re-benchmarked with it yet.
:::

## Recommendations

- **`gemma4:26b` is the default and the best local model.** It shared crisis
  resources in every crisis run, had 100% tool recall, and, as a
  mixture-of-experts model with 4B active parameters, replies several times
  faster than dense models its size.
- **`gemma4:31b` is as good at tools.** It passed one more scenario, but gave
  no crisis line in any run of the harm-to-others scenario.
- **`gemma4:12b` is the smallest model worth trying.** It had 98% tool recall
  but skipped crisis resources in 2 of 8 safety scenarios.
- **Don't use `qwen3.8:27b` (the old default) for now.** It calls tools well
  (88% recall), but often asks whether someone is safe without sharing any
  crisis line, and passed only 2 of 8 safety scenarios.
- **Skip the rest.** `qwen3.6:35b-a3b`, `mistral-small3.2:24b` and
  `nemotron-3.5-lightning:30b` missed many needed tool calls, and
  `nemotron` shared crisis resources in 1 of 8 safety scenarios. Models under
  12B skipped both tool calls and crisis resources.
- **Hosted models trade privacy for quality.** `gpt-5.6-luna` passed every
  scenario and `gemini-3.8-flash` every safety scenario, with full tool
  recall. Both are cheap. Your check-ins leave your machine.
- **Test before switching.** `deepseek-v4.1-flash` once answered anger at
  someone with no crisis line at all, and `claude-sonnet-5` once said it moved
  a step without moving it. `gpt-oss-120b` shared resources every time but
  made changes it should have asked about first (12% of those turns) and
  ignored the prompt's no-dashes rule in a quarter of its replies.

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
  rules such as one question per reply.
- **Tool recall**: turns that needed a tool call and got every required call.
- **Phantom actions**: replies claiming a change that nothing saved.
- **Unwanted writes**: turns where the model changed data it should have asked
  about first.
- **Dashes**: replies with an em or en dash, which the prompt asks the coach
  to avoid. This is style only and doesn't affect the other columns.

Reply times aren't listed, since local ones depend on your hardware; the full
report has them.

## Caveats

- One run per scenario is a smoke test. Models sample differently each time,
  so treat a single pass or fail as a hint, not a verdict.
- The scenarios are synthetic and in English, and use the default prompt and
  tool set.
- Results change with the Ollama version, quantization, the hosting provider
  and settings such as `reasoning_effort`. The same `mistral-small3.2:24b`
  weights passed 21 of 35 scenarios on Ollama and 29 on OpenRouter. The full
  report includes the
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
