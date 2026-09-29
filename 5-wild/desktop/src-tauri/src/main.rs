//! The desktop shell: the web build, unchanged, in the system's webview.
//!
//! Tauri rather than a native port or Electron, for reasons worth keeping. A
//! Godot port ran the engine under QuickJS and redrew every screen from a
//! hand-translated copy of style.css, which made each stylesheet change a
//! change in two places; it was scrapped for that. Electron was built and
//! measured: a 128 MB download, 219 MB of it unpacked being Chromium. Tauri
//! borrows the webview the OS already has, which on Windows is WebView2, the
//! same Chromium the APK's Android WebView is, and on Linux is WebKitGTK, the
//! one engine here that nothing else in the project is tested against.
//!
//! The page gets next to no IPC: `withGlobalTauri` is off, and the one
//! capability granted (capabilities/zoom.json) is the single command Tauri's own
//! zoom script needs on Linux. It is exactly the web build otherwise, and
//! everything else this file does, it does from the Rust side.

// No console window behind the game on Windows. Debug builds keep it for logs.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::webview::NewWindowResponse;
use tauri::{AppHandle, Manager, Url, WebviewUrl, WebviewWindowBuilder};
use tauri_plugin_opener::OpenerExt;

/// Whether a URL is the game's own page. Tauri serves the bundle from
/// `tauri://localhost` on Linux and from `https://tauri.localhost` on Windows
/// (see `use_https_scheme` below).
fn ours(url: &Url) -> bool {
    url.scheme() == "tauri" || url.host_str() == Some("tauri.localhost")
}

/// Anything off the game's own origin goes to the system browser, never a
/// window here: the source link, the bug report form, the credits.
fn external(app: &AppHandle, url: &Url) {
    if url.scheme() == "https" {
        let _ = app.opener().open_url(url.as_str(), None::<&str>);
    }
}

fn main() {
    tauri::Builder::default()
        // Two windows would share one localStorage and write over each other's
        // save; a second launch raises the first instead.
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.unminimize();
                let _ = window.set_focus();
            }
        }))
        .plugin(tauri_plugin_opener::init())
        .setup(|app| {
            let navigating = app.handle().clone();
            let opening = app.handle().clone();
            WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
                .title("5 Wild")
                // Portrait, as the game was laid out: a phone's proportions at a
                // size that fits a 768-pixel-tall laptop screen with its taskbar.
                .inner_size(460.0, 820.0)
                .min_inner_size(360.0, 600.0)
                // Ctrl +/- and Ctrl+wheel, as in a browser tab, which is where
                // anyone who finds the tiles too small has already learned to
                // look. Native on WebView2; on WebKitGTK an injected key handler
                // calling `set_webview_zoom`, hence the capability.
                .zoom_hotkeys_enabled(true)
                // The origin localStorage is keyed on, so it must never change:
                // a new one is a fresh install with the save and the record gone.
                // https for the same reason the APK's is (`androidScheme`), and
                // the telemetry worker's CORS list names it.
                .use_https_scheme(true)
                .on_navigation(move |url| {
                    if ours(url) {
                        return true;
                    }
                    external(&navigating, url);
                    false
                })
                // `target="_blank"` links, which is every outbound link the views
                // draw; see the about sheet in views.ts.
                .on_new_window(move |url, _features| {
                    external(&opening, &url);
                    NewWindowResponse::Deny
                })
                .build()?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("5 Wild failed to start");
}
