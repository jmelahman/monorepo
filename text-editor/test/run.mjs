#!/usr/bin/env node
// Runs the tests in a headless browser and exits with how they went.
//
//   node test/run.mjs
//
// There is nothing to install: this serves the editor from a port of its own,
// opens test/index.html in a browser that is already on the machine, with a
// profile made for the occasion, and waits for the page to send back what
// happened. BROWSER names the one to use where the first found is not wanted.
//
// The same page can be opened by hand from any server of this folder, at
// /test/, to run them where they can be stepped through.

import { execFileSync, spawn } from 'node:child_process';
import { createReadStream, existsSync, mkdtempSync, rmSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { extname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(fileURLToPath(import.meta.url), '..', '..');
const TIMEOUT = 60_000;
const TYPES = {
  '.css': 'text/css',
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2',
};
const BROWSERS = ['firefox', 'chromium', 'chromium-browser', 'google-chrome'];

function onPath(name) {
  try {
    execFileSync('which', [name], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

const browser = process.env.BROWSER ?? BROWSERS.find(onPath);
if (!browser) {
  console.error(`No browser found. Looked for ${BROWSERS.join(', ')}; set BROWSER to another.`);
  process.exit(2);
}

let finish;
const finished = new Promise((done) => { finish = done; });

const server = createServer((request, response) => {
  if (request.method === 'POST' && request.url === '/results') {
    let body = '';
    request.on('data', (chunk) => { body += chunk; });
    request.on('end', () => {
      response.end();
      finish(JSON.parse(body));
    });
    return;
  }
  const path = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
  let file = join(ROOT, path);
  // Nothing above the folder, whatever the address says.
  if (file !== ROOT && !file.startsWith(ROOT + sep)) file = '';
  if (file && existsSync(file) && statSync(file).isDirectory()) file = join(file, 'index.html');
  if (!file || !existsSync(file)) {
    // The editor asks for /files/ to see whether the server has any.
    response.writeHead(404).end();
    return;
  }
  response.writeHead(200, { 'content-type': TYPES[extname(file)] ?? 'application/octet-stream' });
  createReadStream(file).pipe(response);
});

await new Promise((listening) => { server.listen(0, '127.0.0.1', listening); });
const url = `http://127.0.0.1:${server.address().port}/test/`;
const profile = mkdtempSync(join(tmpdir(), 'text-editor-test-'));
const args = /firefox/.test(browser)
  ? ['--headless', '--no-remote', '--profile', profile, url]
  : ['--headless=new', '--no-first-run', '--disable-gpu', `--user-data-dir=${profile}`, url];
const child = spawn(browser, args, { stdio: 'ignore' });
child.on('error', (error) => finish([{ name: `start ${browser}`, error: error.message }]));

const timer = setTimeout(
  () => finish([{ name: 'run the tests', error: `Nothing came back from ${browser} in ${TIMEOUT / 1000} seconds.` }]),
  TIMEOUT,
);
const results = await finished;
clearTimeout(timer);
child.kill();
server.close();
server.closeAllConnections();
rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });

const failed = results.filter((result) => result.error);
for (const { name, error } of results) {
  console.log(error ? `not ok  ${name}\n        ${error.replace(/\n/g, '\n        ')}` : `ok      ${name}`);
}
console.log(`\n${results.length - failed.length} passed, ${failed.length} failed (${browser})`);
process.exit(failed.length ? 1 : 0);
