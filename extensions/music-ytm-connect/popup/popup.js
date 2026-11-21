import { isYouTube, playlistFrom, serverBase, toCookiesTxt } from "../lib/connect.js";

const $ = (id) => document.getElementById(id);
const form = $("server-form");
const input = $("server");
const status = $("status");
const actions = $("actions");
const result = $("result");
const sendCookies = $("send-cookies");
const sendSupermix = $("send-supermix");
const check = $("check");

let base = "";
let tab = null;

function say(el, text, kind = "") {
  el.textContent = text;
  el.className = `${el.id === "status" ? "status" : "result"} ${kind}`.trim();
}

// api calls the music server; the extension holds the server's origin as
// a host permission, so the popup's requests are not held to CORS.
async function api(method, path, body, type = "application/json") {
  const res = await fetch(`${base}${path}`, {
    method,
    headers: body === undefined ? {} : { "Content-Type": type },
    body,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(text.trim() || `${res.status} ${res.statusText}`);
  return text ? JSON.parse(text) : {};
}

function describe(a) {
  const cookies = { saved: "signed in (sent from a browser)", env: "signed in (from the server's environment)" }[a.cookies]
    || "not signed in";
  const supermix = a.supermix ? `Supermix ${a.supermix}` : "no Supermix playlist";
  return `${cookies}; ${supermix}.${a.failed ? ` Last try failed: ${a.failed}` : ""}`;
}

async function refresh() {
  actions.hidden = !base;
  if (!base) {
    say(status, input.value
      ? "Save to let this browser reach the server."
      : "Type the music server's address, as you open it in the browser.");
    return;
  }
  try {
    say(status, describe(await api("GET", "/api/yt/account")), "");
  } catch (err) {
    say(status, `Could not reach ${base}: ${err.message}`, "bad");
  }
}

async function showTab() {
  [tab] = await browser.tabs.query({ active: true, currentWindow: true });
  const onYouTube = tab?.url && /^https:\/\/([a-z0-9-]+\.)*youtube\.com\//.test(tab.url);
  $("cookies-help").textContent = onYouTube
    ? "Sends the YouTube cookies of this window's container, so the server can ask for your mixes."
    : "Open music.youtube.com, signed in, then send.";
  sendSupermix.hidden = !playlistFrom(tab?.url || "");
}

form.addEventListener("submit", (e) => {
  e.preventDefault();
  const typed = serverBase(input.value);
  if (!typed) {
    say(status, "That isn't an http(s) address.", "bad");
    return;
  }
  // The permission is asked for straight from the click: Firefox refuses
  // a request made after anything else has been awaited.
  browser.permissions.request({ origins: [`${new URL(typed).origin}/*`] }).then(async (granted) => {
    if (!granted) {
      say(status, "Without access to the server, nothing can be sent to it.", "bad");
      return;
    }
    base = typed;
    input.value = base;
    await browser.storage.sync.set({ server: base });
    say(result, "");
    await refresh();
  });
});

// youTubeCookies reads YouTube's cookies from the store the tab lives in,
// so a private window or a container sends its own sign-in.
async function youTubeCookies() {
  const query = { domain: "youtube.com" };
  if (tab?.cookieStoreId) query.storeId = tab.cookieStoreId;
  let cookies;
  try {
    cookies = await browser.cookies.getAll(query);
  } catch {
    // First-party isolation insists on being told which party; null is all.
    cookies = await browser.cookies.getAll({ ...query, firstPartyDomain: null });
  }
  return cookies.filter(isYouTube);
}

sendCookies.addEventListener("click", async () => {
  sendCookies.disabled = true;
  say(result, "Sending…");
  try {
    const cookies = await youTubeCookies();
    if (!cookies.some((c) => c.name === "SAPISID" || c.name === "__Secure-3PAPISID")) {
      throw new Error("This window isn't signed in to YouTube; sign in on music.youtube.com first.");
    }
    const a = await api("PUT", "/api/yt/account/cookies", toCookiesTxt(cookies), "text/plain");
    say(status, describe(a));
    say(result, `Sent ${cookies.length} cookies.${a.supermix ? "" : " Now open your Supermix and send it too."}`, "good");
  } catch (err) {
    say(result, err.message, "bad");
  } finally {
    sendCookies.disabled = false;
  }
});

sendSupermix.addEventListener("click", async () => {
  sendSupermix.disabled = true;
  try {
    const a = await api("PUT", "/api/yt/account/supermix", JSON.stringify({ supermix: playlistFrom(tab.url) }));
    say(status, describe(a));
    say(result, "Saved this playlist as your Supermix.", "good");
  } catch (err) {
    say(result, err.message, "bad");
  } finally {
    sendSupermix.disabled = false;
  }
});

check.addEventListener("click", async () => {
  check.disabled = true;
  say(result, "Asking YouTube…");
  try {
    const out = await api("POST", "/api/yt/account/check");
    if (out.ok) say(result, `Signed in: the Supermix has ${out.tracks} tracks.`, "good");
    else say(result, out.error || "YouTube refused the account.", "bad");
  } catch (err) {
    say(result, err.message, "bad");
  } finally {
    check.disabled = false;
    refresh();
  }
});

// The address follows Firefox Sync; the permission to reach it doesn't,
// so each browser is asked once, by Save.
const saved = await browser.storage.sync.get("server");
if (saved.server) {
  const granted = await browser.permissions.contains({ origins: [`${new URL(saved.server).origin}/*`] });
  base = granted ? saved.server : "";
  input.value = saved.server;
}
await Promise.all([showTab(), refresh()]);
