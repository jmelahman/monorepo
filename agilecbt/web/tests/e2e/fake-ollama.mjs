// A scripted stand-in for Ollama's HTTP API so E2E tests can exercise the
// real curator tool loop without a model. Every chat turn asks to add a
// "Short walk" step to Today, then confirms once the tool result comes back.
import { createServer } from "node:http";

const port = Number(process.env.FAKE_OLLAMA_PORT ?? 11499);

function ndjson(res, lines) {
  res.writeHead(200, { "Content-Type": "application/x-ndjson" });
  for (const l of lines) res.write(`${JSON.stringify(l)}\n`);
  res.end();
}

createServer((req, res) => {
  let raw = "";
  req.on("data", (c) => {
    raw += c;
  });
  req.on("end", () => {
    if (req.url === "/api/tags") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ models: [{ name: "fake:latest" }] }));
      return;
    }
    if (req.url !== "/api/chat") {
      res.writeHead(404).end();
      return;
    }
    const body = JSON.parse(raw);
    const msg = (content, extra = {}) => ({
      message: { role: "assistant", content, ...extra },
      done: false,
    });
    if (!body.tools) {
      // One-shot completion: the retro draft.
      const draft = {
        went_well: "You showed up for check-ins.",
        was_hard: "Low energy midweek.",
        try_next: "One short walk after lunch.",
        patterns: "Walks came before better days.",
      };
      ndjson(res, [msg(JSON.stringify(draft)), { ...msg(""), done: true }]);
      return;
    }
    const last = body.messages.at(-1);
    if (last.role === "tool") {
      ndjson(res, [msg("Done, a short walk is on Today. "), { ...msg("Go gently."), done: true }]);
      return;
    }
    ndjson(res, [
      msg("That sounds like a lot. "),
      msg("", {
        tool_calls: [
          {
            function: {
              name: "create_step",
              arguments: { title: "Short walk", lane: "today", energy_cost: 1 },
            },
          },
        ],
      }),
      { ...msg(""), done: true },
    ]);
  });
}).listen(port, "127.0.0.1", () => console.log(`fake ollama on :${port}`));
