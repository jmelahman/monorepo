package com.jmelahman.fivewild;

import android.os.Bundle;
import androidx.core.content.ContextCompat;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {

    @Override
    public void onCreate(Bundle savedInstanceState) {
        // Before super, which is where the bridge is built and its plugins are read.
        registerPlugin(ThemePlugin.class);
        super.onCreate(savedInstanceState);
        // The WebView shows its own background until the page paints, and it
        // has to match the launch window it replaces. That is launchBackground,
        // which follows the night mode ThemePlugin sets; a fixed
        // android.backgroundColor in capacitor.config.ts could only ever match
        // one of the two themes.
        getBridge().getWebView().setBackgroundColor(ContextCompat.getColor(this, R.color.launchBackground));
    }
}
