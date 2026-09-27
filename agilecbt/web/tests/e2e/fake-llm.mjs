// A scripted stand-in for an OpenAI-compatible API so E2E tests can exercise
// the real curator tool loop without a model. Every chat turn asks to add a
// "Short walk" step to Today, then confirms once the tool result comes back.
// The crisis classifier flags only CRISIS_PHRASE, which the built-in phrase
// list misses, so a test exercises the model tier.
import { createServer } from "node:http";

const port = Number(process.env.FAKE_LLM_PORT ?? 11499);
const CRISIS_PHRASE = "rather not wake up";

function sse(res, deltas) {
  res.writeHead(200, { "Content-Type": "text/event-stream" });
  for (const delta of deltas) {
    res.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta }] })}\n\n`);
  }
  res.end("data: [DONE]\n\n");
}

createServer((req, res) => {
  let raw = "";
  req.on("data", (c) => {
    raw += c;
  });
  req.on("end", () => {
    if (req.url === "/v1/models") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ object: "list", data: [{ id: "fake" }] }));
      return;
    }
    if (req.url !== "/v1/chat/completions") {
      res.writeHead(404).end();
      return;
    }
    const body = JSON.parse(raw);
    const system = body.messages[0]?.content ?? "";
    const last = body.messages.at(-1);
    if (system.startsWith("You are a safety classifier")) {
      const category = last.content.includes(CRISIS_PHRASE) ? "suicide" : "none";
      sse(res, [{ role: "assistant", content: JSON.stringify({ category, reason: "fake" }) }]);
      return;
    }
    if (typeof last.content === "string" && last.content.includes("[Safety check:")) {
      // A flagged chat turn: no tools, and no crisis lines, which the app
      // adds itself.
      sse(res, [
        { role: "assistant", content: "I'm really glad you told me. Are you safe right now?" },
      ]);
      return;
    }
    if (!body.tools) {
      // One-shot completion: the retro draft.
      const draft = {
        went_well: "You showed up for check-ins.",
        was_hard: "Low energy midweek.",
        try_next: "One short walk after lunch.",
        patterns: "Walks came before better days.",
      };
      sse(res, [{ role: "assistant", content: JSON.stringify(draft) }]);
      return;
    }
    if (last.role === "tool") {
      sse(res, [{ content: "Done, a short walk is on Today. " }, { content: "Go gently." }]);
      return;
    }
    sse(res, [
      { role: "assistant", content: "That sounds like a lot. " },
      {
        tool_calls: [
          {
            index: 0,
            id: "call_walk",
            type: "function",
            function: {
              name: "create_step",
              arguments: JSON.stringify({ title: "Short walk", lane: "today", energy_cost: 1 }),
            },
          },
        ],
      },
    ]);
  });
}).listen(port, "127.0.0.1", () => console.log(`fake LLM on :${port}`));
