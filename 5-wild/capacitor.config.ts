import type { CapacitorConfig } from "@capacitor/cli"

const config: CapacitorConfig = {
  // Java package segments cannot start with a digit, so the id spells the 5 out.
  // This is the Android install key: changing it later installs a second app
  // beside the first rather than upgrading it.
  appId: "com.jmelahman.fivewild",
  appName: "5 Wild",
  // Must match vite.config.ts build.outDir; guarded by test/build-config.test.ts.
  webDir: "dist",
  // No android.backgroundColor: it is one color and the game has two themes.
  // MainActivity sets the WebView's background from launchBackground, which
  // follows the theme; see ThemePlugin.java.
  server: {
    androidScheme: "https",
    // For live reload on a physical device, uncomment and point at your LAN IP
    // while `bun run dev` is running, then re-run `bunx cap sync android`:
    // url: "http://192.168.1.x:5173",
    // cleartext: true,
  },
}

export default config
