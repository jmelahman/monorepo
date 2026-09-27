# Crisis safety

People write to the coach about low mood and anxiety, and sometimes about
wanting to die. The coach's system prompt tells it how to respond, but the
[benchmarks](/guide/models) showed that following it depends on the model and
doesn't reliably improve with size: models of every size left out the crisis
line, sent an empty reply, or added steps to the board in the middle of a
crisis.

So the app doesn't leave it to the coach model to notice a crisis. Before the
coach replies, a separate crisis classifier checks every chat message. When it
flags one, the app takes over the parts that must never fail.

::: warning Not a clinical tool
The classifier catches most crisis messages, not all of them, and it will
sometimes flag an ordinary one. It's a safety net under the coach, not a
replacement for a person or for emergency services.
:::

## How a message is checked

The check has two tiers:

1. **Phrase list.** A built-in list of explicit phrases, such as "kill
   myself", "cut myself again" or "saving up my pills". It runs in the server,
   adds no delay, and is always on. It's tuned so it doesn't fire on figures
   of speech like "this deadline is killing me", so it misses indirect
   phrasing.
2. **Model (opt-in).** If you set a `safety_model` and the phrase list
   doesn't match, a short classification call asks that model whether the message (with the last few messages for context)
   suggests suicide, self-harm, harm to someone else, or immediate danger. The
   model tier catches passive ideation ("everyone would be better off without
   me"), jokes ("I'll just unalive myself lol") and a bare "no" after "are you
   safe right now?". It costs one extra request per message.

Without a model tier, anything the phrase list misses is left to the coach's
prompt, as it was before the classifier.

If the model call fails or takes more than 20 seconds, the app uses the phrase
list's verdict and logs a warning. It fails open like this because failing
closed would put every message into crisis mode while the model is down.

## What happens when a message is flagged

The coach still writes the reply, but:

- **No tools.** The coach can't add or move steps, save memories or touch the
  board on that turn.
- **A safety note.** The coach's turn includes a note saying the app flagged
  the message, what kind of crisis it may be, and what to do first. For
  example, if someone may be in danger, getting safe comes first.
- **The crisis lines always arrive.** If the reply doesn't include any of your
  [crisis resources](/guide/configuration#crisis-resources), the app appends
  them. If someone may be in danger and the reply has no emergency number, it
  adds one. If the coach fails or says nothing, the app sends a short,
  scripted reply with the resources.
- **A support card.** The chat shows a short card under the reply. It points
  to emergency services and links to your full list of crisis lines in
  Settings → Support. The card is saved with the message, so it's still there
  after a reload.

## Adding a classifier model

By default only the phrase list runs, so each message is still a single
request to the coach. The phrase list catches explicit phrasing but misses
most indirect messages: on the labeled benchmark messages it catches 20 of 49,
while a small model catches all of them.

To add the model tier, set `safety_model` in
[`config.toml`](/guide/configuration#crisis-classifier):

```toml
safety_model = "gemma4:e4b"
```

It uses the coach's endpoint and key unless you also set `safety_base_url`
(and `safety_api_key`). Setting it to the coach's own model works too, and
needs no extra memory.

Each check is a small request, but it runs before the coach replies, so it
adds its latency to every message. A small model keeps that short:
`gemma4:e4b` took about 2 seconds per check on a local GPU. It's also the safer
choice on a busy or slow server. When the check takes longer than 20 seconds,
the app falls back to the phrase list, and the server logs
`safety: classifier failed`. With Ollama, make sure the classifier and the
coach fit in memory together. Otherwise each message waits for a model swap,
and a check that waits too long for its model to load falls back to the phrase
list.

Before picking a model, compare candidates on the labeled messages in
`evals/safety/cases.toml`:

```sh
agilecbt eval classifier --model gemma4:e4b --model qwen3.5:9b
```

For each tier it prints the share of crisis messages caught (overall and per
category), false alarms on ordinary messages, and latency. Missing a crisis
message matters more than a false alarm. A false alarm costs one turn without
tools and an unneeded support card. See
[Coach benchmarks](/guide/benchmarks#crisis-classifier).

## Limits

- The phrase list stays quiet on everyday venting like "I'm gonna kill my
  brother", so it only flags a threat to someone else when it's made explicit
  ("I'm serious", "planning how to"). Without a `safety_model`, the rest is up
  to the coach's prompt.
- It covers the in-app coach. [MCP](/guide/ai) clients use their own model and
  aren't checked.
- It reads English phrasing best. The phrase list is English only, and
  small models are weaker in other languages.
- With `llm = "none"` there's no coach, so there's nothing to check.
