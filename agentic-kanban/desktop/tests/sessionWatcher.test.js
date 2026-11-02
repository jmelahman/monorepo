import { afterEach, describe, expect, test } from "bun:test";
import { SessionWatcher } from "../src/sessionWatcher.js";

// A real HTTP server speaking the kanban API's shape, so the watcher's streaming
// and JSON handling run against actual responses.
function startApi() {
  const state = {
    boards: [{ id: 1, name: "Main" }],
    sessions: { 1: [{ id: 10, ticket_id: 100, status: "working" }] },
    streams: new Set(),
    streamUrls: [],
    connected: Promise.withResolvers(),
  };
  const encoder = new TextEncoder();
  const server = Bun.serve({
    port: 0,
    idleTimeout: 0,
    fetch(req) {
      const { pathname, search } = new URL(req.url);
      if (pathname === "/api/boards") return Response.json(state.boards);
      const stateMatch = pathname.match(/^\/api\/boards\/(\d+)\/state$/);
      if (stateMatch) return Response.json({ sessions: state.sessions[stateMatch[1]] ?? [] });
      if (pathname === "/api/tickets/100") return Response.json({ title: "Fix the thing" });
      if (pathname === "/api/events") {
        state.streamUrls.push(search);
        let controller;
        const body = new ReadableStream({
          start(c) {
            controller = c;
            state.streams.add(c);
            state.connected.resolve();
          },
          cancel() {
            state.streams.delete(controller);
          },
        });
        return new Response(body, { headers: { "Content-Type": "text/event-stream" } });
      }
      return new Response("not found", { status: 404 });
    },
  });
  state.send = (boardId, session) => {
    const payload = JSON.stringify({ type: "session_updated", data: session, board_id: boardId });
    for (const c of state.streams) {
      c.enqueue(encoder.encode(`event: session_updated\ndata: ${payload}\n\n`));
    }
  };
  state.url = `http://localhost:${server.port}`;
  state.close = () => {
    for (const c of state.streams) c.close();
    server.stop(true);
  };
  return state;
}

async function waitFor(check) {
  const deadline = Date.now() + 2_000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error("timed out");
    await Bun.sleep(5);
  }
}

describe("SessionWatcher", () => {
  let api;
  let watcher;

  function start(options = {}) {
    api = startApi();
    const notified = [];
    watcher = new SessionWatcher({
      server: api.url,
      fetch,
      notify: (n) => notified.push(n),
      pollMs: 60_000,
      retryMs: 10,
      ...options,
    });
    void watcher.run();
    return notified;
  }

  afterEach(() => {
    watcher.stop();
    api.close();
  });

  test("announces a transition from the stream with the ticket and board", async () => {
    const notified = start();
    await api.connected.promise;
    api.send(1, { id: 10, ticket_id: 100, status: "awaiting_perm" });
    await waitFor(() => notified.length === 1);
    expect(notified[0]).toEqual({
      event: "awaiting_perm",
      title: "Fix the thing",
      body: "Waiting for permission · Main",
      sessionId: 10,
      boardId: 1,
      ticketId: 100,
    });
  });

  test("repeated statuses notify once", async () => {
    const notified = start();
    await api.connected.promise;
    api.send(1, { id: 10, ticket_id: 100, status: "idle" });
    api.send(1, { id: 10, ticket_id: 100, status: "idle" });
    await waitFor(() => notified.length >= 1);
    await Bun.sleep(30);
    expect(notified.map((n) => n.event)).toEqual(["finished"]);
  });

  test("old failures present at launch stay quiet", async () => {
    const notified = start();
    api.sessions[1] = [{ id: 11, ticket_id: 100, status: "error" }];
    await api.connected.promise;
    await Bun.sleep(30);
    expect(notified).toEqual([]);
  });

  test("polling catches a failure that was never streamed", async () => {
    const notified = start({ pollMs: 20 });
    await api.connected.promise;
    api.sessions[1] = [{ id: 10, ticket_id: 100, status: "error" }];
    await waitFor(() => notified.length === 1);
    expect(notified[0].event).toBe("error");
  });

  test("a new board restarts the stream to include it", async () => {
    start({ pollMs: 20 });
    await api.connected.promise;
    api.boards.push({ id: 2, name: "Other" });
    await waitFor(() => api.streamUrls.includes("?boards=1,2"));
  });

  test("a ticket lookup failure still notifies", async () => {
    const notified = start();
    await api.connected.promise;
    api.send(1, { id: 12, ticket_id: 999, status: "error" });
    await waitFor(() => notified.length === 1);
    expect(notified[0].title).toBe("Ticket #999");
  });

  test("reconnects after the stream drops", async () => {
    const notified = start();
    await api.connected.promise;
    for (const c of api.streams) c.close();
    api.streams.clear();
    await waitFor(() => api.streams.size === 1);
    api.send(1, { id: 10, ticket_id: 100, status: "idle" });
    await waitFor(() => notified.length === 1);
    expect(notified[0].event).toBe("finished");
  });
});
