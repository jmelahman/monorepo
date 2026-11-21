import { test } from "node:test";
import assert from "node:assert/strict";
import { isYouTube, playlistFrom, serverBase, toCookiesTxt } from "../lib/connect.js";

test("toCookiesTxt writes the Netscape format", () => {
  const txt = toCookiesTxt([
    { domain: ".youtube.com", hostOnly: false, path: "/", secure: true, httpOnly: false,
      expirationDate: 1893456000.5, name: "SAPISID", value: "a" },
    { domain: "music.youtube.com", hostOnly: true, path: "/", secure: true, httpOnly: true,
      session: true, name: "HSID", value: "b" },
    { domain: "youtube.com", hostOnly: false, path: "", secure: false, httpOnly: false,
      expirationDate: 1893456000, name: "PREF", value: "c" },
  ]);
  const lines = txt.split("\n");
  assert.equal(lines[0], "# Netscape HTTP Cookie File");
  assert.ok(lines.includes(".youtube.com\tTRUE\t/\tTRUE\t1893456000\tSAPISID\ta"));
  assert.ok(lines.includes("#HttpOnly_music.youtube.com\tFALSE\t/\tTRUE\t0\tHSID\tb"));
  assert.ok(lines.includes(".youtube.com\tTRUE\t/\tFALSE\t1893456000\tPREF\tc"));
  assert.ok(txt.endsWith("\n"));
});

test("isYouTube takes YouTube and its subdomains only", () => {
  assert.ok(isYouTube({ domain: ".youtube.com" }));
  assert.ok(isYouTube({ domain: "music.youtube.com" }));
  assert.ok(!isYouTube({ domain: ".notyoutube.com" }));
  assert.ok(!isYouTube({ domain: "youtube.com.evil.example" }));
});

test("playlistFrom reads list= from YouTube addresses", () => {
  const id = "RDTMAK5uy_kset8DisdE7LSD4TNjEVvrKRTmG7a56sY";
  assert.equal(playlistFrom(`https://music.youtube.com/playlist?list=${id}`), id);
  assert.equal(playlistFrom(`https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=${id}`), id);
  assert.equal(playlistFrom("https://music.youtube.com/watch?v=dQw4w9WgXcQ"), "");
  assert.equal(playlistFrom(`https://example.com/playlist?list=${id}`), "");
  assert.equal(playlistFrom("https://music.youtube.com/playlist?list=short"), "");
  assert.equal(playlistFrom("not a url"), "");
});

test("serverBase keeps the path the app is served under", () => {
  assert.equal(serverBase("homelab/music"), "http://homelab/music");
  assert.equal(serverBase(" http://homelab/music/ "), "http://homelab/music");
  assert.equal(serverBase("https://homelab.example/music/#/album/x"), "https://homelab.example/music");
  assert.equal(serverBase("http://homelab:8095/index.html"), "http://homelab:8095");
  assert.equal(serverBase("http://homelab:8095"), "http://homelab:8095");
  assert.equal(serverBase(""), "");
  assert.equal(serverBase("ftp://homelab/music"), "");
});
