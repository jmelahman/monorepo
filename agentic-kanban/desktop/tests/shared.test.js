import { describe, expect, test } from "bun:test";
import {
  DEFAULT_SERVER,
  SseParser,
  isSameOrigin,
  isWebUrl,
  notificationEvent,
  resolveServer,
} from "../src/shared.js";

describe("resolveServer", () => {
  test("uses the first source that is set", () => {
    expect(resolveServer("", undefined, "http://devbox:7474/")).toBe("http://devbox:7474");
    expect(resolveServer("http://a:1", "http://b:2")).toBe("http://a:1");
  });

  test("falls back to the local server", () => {
    expect(resolveServer("", undefined, undefined)).toBe(DEFAULT_SERVER);
  });

  test("rejects anything but http and https", () => {
    expect(() => resolveServer("file:///etc/passwd")).toThrow();
    expect(() => resolveServer("devbox")).toThrow();
  });
});

describe("URL checks", () => {
  test("isWebUrl only accepts http and https", () => {
    expect(isWebUrl("https://example.com")).toBe(true);
    expect(isWebUrl("javascript:alert(1)")).toBe(false);
    expect(isWebUrl("file:///etc/passwd")).toBe(false);
    expect(isWebUrl("")).toBe(false);
  });

  test("isSameOrigin compares scheme, host and port", () => {
    const origin = "http://localhost:7474";
    expect(isSameOrigin("http://localhost:7474/api/boards", origin)).toBe(true);
    expect(isSameOrigin("http://localhost:13000/", origin)).toBe(false);
    expect(isSameOrigin("http://localhost:7474.evil.example/", origin)).toBe(false);
    expect(isSameOrigin("not a url", origin)).toBe(false);
  });
});

describe("notificationEvent", () => {
  test.each([
    ["working", "awaiting_perm", "awaiting_perm"],
    ["working", "idle", "finished"],
    ["awaiting_perm", "idle", "finished"],
    ["starting", "error", "error"],
    ["working", "error", "error"],
    [undefined, "error", "error"],
  ])("%s -> %s notifies %s", (previous, next, event) => {
    expect(notificationEvent(previous, next)).toBe(event);
  });

  test.each([
    ["idle", "awaiting_perm"],
    ["working", "working"],
    ["error", "error"],
    ["working", "stopped"],
    ["starting", "working"],
    [undefined, "idle"],
    [undefined, "awaiting_perm"],
  ])("%s -> %s stays quiet", (previous, next) => {
    expect(notificationEvent(previous, next)).toBeNull();
  });
});

describe("SseParser", () => {
  test("parses named events and skips comments", () => {
    const events = new SseParser().push(': ping\n\nevent: session_updated\ndata: {"a":1}\n\n');
    expect(events).toEqual([{ event: "session_updated", data: '{"a":1}' }]);
  });

  test("holds a partial event until the rest arrives", () => {
    const parser = new SseParser();
    expect(parser.push("event: x\nda")).toEqual([]);
    expect(parser.push("ta: 1\n")).toEqual([]);
    expect(parser.push("\n")).toEqual([{ event: "x", data: "1" }]);
  });

  test("handles a CRLF split across chunks", () => {
    const parser = new SseParser();
    expect(parser.push("data: 1\r\n\r")).toEqual([]);
    expect(parser.push("\n")).toEqual([{ event: "message", data: "1" }]);
  });

  test("joins multi-line data", () => {
    expect(new SseParser().push("data: a\ndata: b\n\n")).toEqual([
      { event: "message", data: "a\nb" },
    ]);
  });
});
