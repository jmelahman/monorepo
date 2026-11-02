import { SseParser, eventText, notificationEvent } from "./shared.js";

const POLL_MS = 30_000;
const RETRY_MS = 5_000;

function sleep(ms, signal) {
  return new Promise((resolve) => {
    if (signal.aborted) return resolve();
    const timer = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => (clearTimeout(timer), resolve()), { once: true });
  });
}

// Reports session status changes for desktop notifications. Web Push isn't
// available in Electron, so this follows the same stream the board UI does.
//
// The stream gives prompt updates, but a failed session start only changes the
// row without publishing an event, and a dropped connection misses whatever
// happened meanwhile. Polling the board state catches both.
export class SessionWatcher {
  statuses = new Map();
  boards = new Map();
  stopper = new AbortController();

  constructor({ server, fetch, notify, pollMs = POLL_MS, retryMs = RETRY_MS }) {
    Object.assign(this, { server, fetch, notify, pollMs, retryMs });
  }

  async run() {
    const { signal } = this.stopper;
    while (!signal.aborted) {
      try {
        await this.sync();
        await this.follow();
      } catch (err) {
        if (!signal.aborted) console.warn(`session watcher: ${err.message}`);
      }
      await sleep(this.retryMs, signal);
    }
  }

  stop() {
    this.stopper.abort();
  }

  async getJson(path, signal = this.stopper.signal) {
    const res = await this.fetch(`${this.server}${path}`, { signal, cache: "no-store" });
    if (!res.ok) throw new Error(`GET ${path}: ${res.status}`);
    return res.json();
  }

  // Brings statuses up to date. Sessions on a board seen for the first time
  // only set the baseline, or launching the app would announce old failures.
  async sync() {
    const boards = await this.getJson("/api/boards");
    await Promise.all(
      boards.map(async (board) => {
        const state = await this.getJson(`/api/boards/${board.id}/state`);
        const known = this.boards.has(board.id);
        for (const session of state.sessions ?? []) {
          if (known) this.observe(session, board);
          else this.statuses.set(session.id, session.status);
        }
      }),
    );
    this.boards = new Map(boards.map((board) => [board.id, board]));
  }

  boardKey() {
    return [...this.boards.keys()].sort((a, b) => a - b).join(",");
  }

  // Streams until the connection drops or the set of boards changes.
  async follow() {
    const key = this.boardKey();
    const stream = new AbortController();
    const onStop = () => stream.abort();
    this.stopper.signal.addEventListener("abort", onStop, { once: true });
    const poll = setInterval(() => {
      this.sync()
        .then(() => {
          if (this.boardKey() !== key) stream.abort();
        })
        .catch((err) => {
          if (!stream.signal.aborted) console.warn(`session watcher: ${err.message}`);
        });
    }, this.pollMs);
    try {
      if (key === "") await sleep(this.pollMs, stream.signal);
      else await this.stream(key, stream.signal);
    } finally {
      clearInterval(poll);
      this.stopper.signal.removeEventListener("abort", onStop);
    }
  }

  async stream(key, signal) {
    const res = await this.fetch(`${this.server}/api/events?boards=${key}`, {
      signal,
      cache: "no-store",
      headers: { Accept: "text/event-stream" },
    });
    if (!res.ok || !res.body) throw new Error(`GET /api/events: ${res.status}`);
    const parser = new SseParser();
    const decoder = new TextDecoder();
    try {
      for await (const chunk of res.body) {
        for (const { event, data } of parser.push(decoder.decode(chunk, { stream: true }))) {
          if (event === "session_updated") this.onSessionUpdated(data);
        }
      }
    } catch (err) {
      if (!signal.aborted) throw err;
    }
  }

  onSessionUpdated(raw) {
    let message;
    try {
      message = JSON.parse(raw);
    } catch {
      return;
    }
    const board = this.boards.get(message.board_id);
    if (board && message.data) this.observe(message.data, board);
  }

  observe(session, board) {
    const previous = this.statuses.get(session.id);
    this.statuses.set(session.id, session.status);
    const event = notificationEvent(previous, session.status);
    if (event) void this.announce(event, session, board);
  }

  async announce(event, session, board) {
    let title = `Ticket #${session.ticket_id}`;
    try {
      const ticket = await this.getJson(`/api/tickets/${session.ticket_id}`);
      if (ticket.title) title = ticket.title;
    } catch {
      // A notification without the ticket's title beats no notification.
    }
    this.notify({
      event,
      title,
      body: `${eventText(event)} · ${board.name}`,
      sessionId: session.id,
      boardId: board.id,
      ticketId: session.ticket_id,
    });
  }
}
