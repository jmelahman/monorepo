// The pure parts of the extension, kept apart from the browser APIs so
// they test under Bun: the cookies.txt the server takes, the Supermix id
// in a page's address, and the server address the user types.

// toCookiesTxt writes cookies (as browser.cookies.getAll returns them) in
// the Netscape format yt-dlp reads, which is what the music server's
// PUT /api/yt/account/cookies checks for.
export function toCookiesTxt(cookies) {
  const lines = [
    "# Netscape HTTP Cookie File",
    "# Sent by Music Server Connect for YouTube Music.",
    "",
  ];
  for (const c of cookies) {
    // A host-only cookie is written without the leading dot and with the
    // subdomain flag off; a domain cookie with both.
    const domain = c.hostOnly
      ? c.domain.replace(/^\./, "")
      : c.domain.startsWith(".")
        ? c.domain
        : `.${c.domain}`;
    lines.push(
      [
        (c.httpOnly ? "#HttpOnly_" : "") + domain,
        c.hostOnly ? "FALSE" : "TRUE",
        c.path || "/",
        c.secure ? "TRUE" : "FALSE",
        // Session cookies have no expiry; 0 is how the format says so.
        String(c.session || !c.expirationDate ? 0 : Math.floor(c.expirationDate)),
        c.name,
        c.value,
      ].join("\t"),
    );
  }
  return lines.join("\n") + "\n";
}

// isYouTube is whether a cookie belongs to YouTube itself, rather than a
// site whose name merely ends the same way.
export function isYouTube(cookie) {
  const d = cookie.domain.replace(/^\./, "");
  return d === "youtube.com" || d.endsWith(".youtube.com");
}

// playlistFrom is the playlist id in a YouTube Music address (a playlist
// page, or a song playing from one), or "" when there is none.
export function playlistFrom(url) {
  try {
    const u = new URL(url);
    if (!/(^|\.)youtube\.com$/.test(u.hostname)) return "";
    const id = u.searchParams.get("list") || "";
    return /^[A-Za-z0-9_-]{10,64}$/.test(id) ? id : "";
  } catch {
    return "";
  }
}

// serverBase reads the address typed for the music server, with or
// without a scheme, as the base its API hangs off: the homelab serves it
// under /music/, so the path is kept. Plain http unless https is typed,
// since the server lives on the home network.
export function serverBase(typed) {
  let s = typed.trim();
  if (!s) return "";
  if (!/^[a-z]+:\/\//i.test(s)) s = `http://${s}`;
  try {
    const u = new URL(s);
    if (u.protocol !== "http:" && u.protocol !== "https:") return "";
    // The page's own address works too: the hash route goes with the
    // URL, a file (index.html) is dropped, and the folder is kept.
    const path = u.pathname.replace(/\/[^/]*\.[^/]*$/, "").replace(/\/+$/, "");
    return u.origin + path;
  } catch {
    return "";
  }
}
