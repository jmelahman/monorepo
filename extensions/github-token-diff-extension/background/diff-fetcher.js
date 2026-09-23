// Fetches https://github.com/{owner}/{repo}/pull/{n}.diff — which 30x-redirects
// to patch-diff.githubusercontent.com. Must run in the background script:
// github.com's page CSP does not allow-list the redirect host, and Firefox
// applies page CSP to content-script fetches.

const MAX_DIFF_BYTES = 8 * 1024 * 1024;

export async function fetchDiff(owner, repo, prNumber, { etag } = {}) {
  const url = `https://github.com/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/pull/${encodeURIComponent(prNumber)}.diff`;
  const headers = { Accept: "text/plain" };
  if (etag) headers["If-None-Match"] = etag;

  let res;
  try {
    // credentials: "include" — private-repo access rides on the github.com
    // session cookie; extension fetches don't send it by default.
    res = await fetch(url, { credentials: "include", redirect: "follow", headers });
  } catch {
    return { error: "NETWORK" };
  }
  if (res.status === 304) return { notModified: true };
  if (!res.ok) return { error: `HTTP_${res.status}` };

  // Responses are chunked with no Content-Length — guard size while streaming.
  const reader = res.body.getReader();
  const chunks = [];
  let bytes = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    bytes += value.byteLength;
    if (bytes > MAX_DIFF_BYTES) {
      await reader.cancel();
      return { error: "TOO_LARGE" };
    }
    chunks.push(value);
  }
  const merged = new Uint8Array(bytes);
  let offset = 0;
  for (const c of chunks) {
    merged.set(c, offset);
    offset += c.byteLength;
  }
  return { text: new TextDecoder().decode(merged), etag: res.headers.get("etag") };
}
