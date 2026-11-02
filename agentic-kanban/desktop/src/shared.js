// Logic with no Electron dependency, so `bun test` can cover it directly.

export const DEFAULT_SERVER = "http://localhost:7474";

// The origin of the first configured server, most specific source first. The
// frontend calls root-relative paths, so a server can't live under a subpath.
export function resolveServer(...candidates) {
  const raw = candidates.find(Boolean) ?? DEFAULT_SERVER;
  let url;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`Server URL "${raw}" must look like http://host:port`);
  }
  if (!isWebUrl(raw)) throw new Error(`Server URL "${raw}" must look like http://host:port`);
  return url.origin;
}

export function isWebUrl(raw) {
  try {
    const { protocol } = new URL(raw);
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
}

export function isSameOrigin(raw, origin) {
  try {
    return new URL(raw).origin === origin;
  } catch {
    return false;
  }
}

// Which notification a status change deserves, if any. Mirrors
// updateSessionStatus in internal/api/handlers.go: hooks re-report the same
// status freely, and idle → awaiting_perm is Claude's idle reminder rather
// than a real prompt. See REGRESSIONS.md: "Status hooks repeat, so push
// notifications key off the transition".
export function notificationEvent(previous, next) {
  if (previous === next) return null;
  if (next === "error") return "error";
  if (next === "awaiting_perm" && previous === "working") return "awaiting_perm";
  if (next === "idle" && (previous === "working" || previous === "awaiting_perm")) {
    return "finished";
  }
  return null;
}

const EVENT_TEXT = {
  awaiting_perm: "Waiting for permission",
  finished: "Finished",
  error: "Session stopped unexpectedly",
};

export function eventText(event) {
  return EVENT_TEXT[event] ?? event;
}

// Splits a Server-Sent Events byte stream's decoded text into events. Chunks
// can end anywhere, including between a CR and its LF.
export class SseParser {
  buffer = "";

  // Returns the events completed by this chunk, as { event, data }.
  push(chunk) {
    this.buffer += chunk;
    const normalized = this.buffer.replace(/\r\n?/g, "\n");
    // A trailing CR may be half of a CRLF, so leave it for the next chunk.
    const heldBack = this.buffer.endsWith("\r") ? "\r" : "";
    const text = heldBack ? normalized.slice(0, -1) : normalized;
    const blocks = text.split("\n\n");
    this.buffer = blocks.pop() + heldBack;
    return blocks.flatMap((block) => {
      const parsed = parseBlock(block);
      return parsed ? [parsed] : [];
    });
  }
}

function parseBlock(block) {
  let event = "message";
  const data = [];
  for (const line of block.split("\n")) {
    if (line.startsWith(":")) continue;
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    const value = colon === -1 ? "" : line.slice(colon + 1).replace(/^ /, "");
    if (field === "event") event = value;
    else if (field === "data") data.push(value);
  }
  return data.length > 0 ? { event, data: data.join("\n") } : null;
}
