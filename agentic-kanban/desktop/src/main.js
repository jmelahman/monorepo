import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import {
  BrowserWindow,
  Menu,
  Notification,
  app,
  dialog,
  net,
  session as electronSession,
  shell,
} from "electron";
import { SessionWatcher } from "./sessionWatcher.js";
import { isSameOrigin, isWebUrl, resolveServer } from "./shared.js";

const RECONNECT_MS = 2_000;
const ASSETS = join(import.meta.dirname, "..", "assets");
// What the page may ask for. Everything else (camera, location, ...) is denied.
const ALLOWED_PERMISSIONS = new Set(["clipboard-read", "clipboard-sanitized-write"]);

app.setPath("userData", join(app.getPath("appData"), "kanban-desktop"));
// Lets the frontend say that notifications come from this app (web/src/push.ts).
app.userAgentFallback = `${app.userAgentFallback} KanbanDesktop/${app.getVersion()}`;

const settingsPath = join(app.getPath("userData"), "settings.json");

function loadSettings() {
  try {
    const parsed = JSON.parse(readFileSync(settingsPath, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function saveSettings(settings) {
  mkdirSync(app.getPath("userData"), { recursive: true });
  writeFileSync(settingsPath, `${JSON.stringify(settings, null, 2)}\n`);
}

const settings = loadSettings();
let server;
try {
  server = resolveServer(
    app.commandLine.getSwitchValue("server"),
    process.env.KANBAN_URL,
    settings.server,
  );
} catch (err) {
  dialog.showErrorBox("Agentic Kanban", err.message);
  app.exit(1);
}

let watcher = null;
// One per session, so a newer status replaces the older one. Holding the
// reference also keeps the click handler from being garbage collected.
const notifications = new Map();

function openExternal(url) {
  if (isWebUrl(url)) void shell.openExternal(url);
}

async function serverIsUp() {
  try {
    const res = await net.fetch(`${server}/api/health`, { cache: "no-store" });
    return res.ok;
  } catch {
    return false;
  }
}

// Loads path once the server answers, showing a waiting page until then.
async function loadApp(win, path = "/") {
  clearTimeout(win.reconnectTimer);
  if (win.isDestroyed()) return;
  if (await serverIsUp()) {
    if (!win.isDestroyed()) await win.loadURL(`${server}${path}`).catch(() => {});
    return;
  }
  if (win.isDestroyed()) return;
  if (!win.webContents.getURL().startsWith("file:")) {
    await win.loadFile(join(import.meta.dirname, "offline.html"), { query: { server } });
  }
  win.reconnectTimer = setTimeout(() => void loadApp(win, path), RECONNECT_MS);
}

function showContextMenu(win, params) {
  const items = [];
  if (isWebUrl(params.linkURL)) {
    items.push(
      { label: "Open Link in Browser", click: () => openExternal(params.linkURL) },
      { type: "separator" },
    );
  }
  if (params.isEditable) {
    items.push({ role: "cut" }, { role: "copy" }, { role: "paste" }, { role: "selectAll" });
  } else if (params.selectionText) {
    items.push({ role: "copy" });
  }
  if (items.at(-1)?.type === "separator") items.pop();
  if (items.length > 0) Menu.buildFromTemplate(items).popup({ window: win });
}

function createWindow(path = "/") {
  const win = new BrowserWindow({
    width: 1400,
    height: 900,
    autoHideMenuBar: true,
    backgroundColor: "#09090b",
    icon: join(ASSETS, "icon.png"),
  });
  const { webContents } = win;

  // Previews, forwarded ports and PR links belong in the real browser.
  webContents.setWindowOpenHandler(({ url }) => {
    openExternal(url);
    return { action: "deny" };
  });
  const keepOnServer = (event, url) => {
    if (isSameOrigin(url, server)) return;
    event.preventDefault();
    openExternal(url);
  };
  webContents.on("will-navigate", keepOnServer);
  webContents.on("will-redirect", keepOnServer);
  webContents.on("did-fail-load", (_event, code, _description, url, isMainFrame) => {
    // -3 is ERR_ABORTED: a navigation replaced this one.
    if (isMainFrame && code !== -3 && isSameOrigin(url, server)) void loadApp(win);
  });
  webContents.on("context-menu", (_event, params) => showContextMenu(win, params));
  win.on("closed", () => clearTimeout(win.reconnectTimer));

  void loadApp(win, path);
  return win;
}

function focusWindow(path) {
  const win = BrowserWindow.getAllWindows()[0];
  if (!win) return createWindow(path);
  if (win.isMinimized()) win.restore();
  win.focus();
  if (path) void loadApp(win, path);
  return win;
}

function showNotification({ title, body, sessionId, boardId, ticketId }) {
  if (!Notification.isSupported()) return;
  notifications.get(sessionId)?.close();
  const notification = new Notification({ title, body, icon: join(ASSETS, "icon.png") });
  // web/src/push.ts turns these parameters into opening the ticket.
  notification.on("click", () => focusWindow(`/?board=${boardId}&ticket=${ticketId}`));
  notification.on("close", () => {
    if (notifications.get(sessionId) === notification) notifications.delete(sessionId);
  });
  notifications.set(sessionId, notification);
  notification.show();
}

function setNotifications(enabled) {
  watcher?.stop();
  watcher = null;
  if (enabled) {
    watcher = new SessionWatcher({
      server,
      fetch: (url, init) => net.fetch(url, init),
      notify: showNotification,
    });
    void watcher.run();
  }
}

// Replaces Electron's default menu, whose Ctrl+W, Ctrl+R and Ctrl+Q would
// otherwise take those keys from the session terminal. Shortcuts here stick to
// combinations a terminal doesn't need, like a terminal emulator's own do.
function buildMenu() {
  return Menu.buildFromTemplate([
    {
      label: "File",
      submenu: [
        {
          label: "New Window",
          accelerator: "Ctrl+Shift+N",
          click: () => createWindow(),
        },
        { role: "close", accelerator: "Ctrl+Shift+W" },
        { role: "quit", accelerator: "Ctrl+Shift+Q" },
      ],
    },
    {
      label: "View",
      submenu: [
        { role: "reload", accelerator: "Ctrl+Shift+R" },
        { role: "toggleDevTools", accelerator: "Ctrl+Shift+I" },
        { type: "separator" },
        { role: "resetZoom" },
        { role: "zoomIn", accelerator: "Ctrl+=" },
        { role: "zoomOut" },
        { type: "separator" },
        { role: "togglefullscreen" },
        { type: "separator" },
        {
          label: "Show Notifications",
          type: "checkbox",
          checked: settings.notifications !== false,
          click: ({ checked }) => {
            settings.notifications = checked;
            saveSettings(settings);
            setNotifications(checked);
          },
        },
      ],
    },
  ]);
}

function restrictPermissions() {
  const { defaultSession } = electronSession;
  defaultSession.setPermissionRequestHandler((_contents, permission, callback, details) => {
    const url = details.requestingUrl;
    callback(ALLOWED_PERMISSIONS.has(permission) && isSameOrigin(url, server));
  });
  defaultSession.setPermissionCheckHandler(
    (_contents, permission, origin) =>
      ALLOWED_PERMISSIONS.has(permission) && isSameOrigin(origin, server),
  );
}

// A second copy would double every notification.
if (server && !app.requestSingleInstanceLock()) {
  app.quit();
} else if (server) {
  app.on("second-instance", () => focusWindow());
  app.on("window-all-closed", () => app.quit());
  app.on("before-quit", () => watcher?.stop());
  app.whenReady().then(() => {
    restrictPermissions();
    Menu.setApplicationMenu(buildMenu());
    createWindow();
    setNotifications(settings.notifications !== false);
  });
}
