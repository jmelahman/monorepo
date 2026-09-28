package com.jmelahman.fivewild;

import android.app.UiModeManager;
import android.content.Context;
import android.os.Build;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

/**
 * The in-game theme setting, handed to Android so the next launch opens in it.
 *
 * The page applies the theme itself, before its first paint, but the launch
 * window and the WebView's own background are drawn before the page exists,
 * from the launchBackground resource, which picks values/ or values-notnight/ by
 * the night mode in force. Left to the system that is the device's mode, so a
 * player who chose Light on a dark phone saw a dark launch cut to a light page.
 *
 * setApplicationNightMode is the one lever the starting window listens to: the
 * system persists it per app and applies it before any of this process's code
 * runs. It is Android 12 and up; below that the launch follows the device, and
 * the setting still takes over as soon as the page loads.
 */
@CapacitorPlugin(name = "Theme")
public class ThemePlugin extends Plugin {

    @PluginMethod
    public void set(PluginCall call) {
        String theme = call.getString("theme", "system");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            UiModeManager manager = (UiModeManager) getContext().getSystemService(Context.UI_MODE_SERVICE);
            int mode = "light".equals(theme)
                ? UiModeManager.MODE_NIGHT_NO
                : "dark".equals(theme) ? UiModeManager.MODE_NIGHT_YES : UiModeManager.MODE_NIGHT_AUTO;
            // Not compared against getNightMode() first: that answers for the
            // device, not this app, so it would skip a Light chosen on a light
            // phone and leave the override stuck on an earlier Dark. Setting the
            // mode already in force changes no configuration.
            if (manager != null) manager.setApplicationNightMode(mode);
        }
        call.resolve();
    }
}
