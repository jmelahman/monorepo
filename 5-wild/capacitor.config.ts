import type { CapacitorConfig } from "@capacitor/cli"

const config: CapacitorConfig = {
  // Java package segments cannot start with a digit, so the id spells the 5 out.
  // This is the Android install key: changing it later installs a second app
  // beside the first rather than upgrading it.
  appId: "com.jmelahman.fivewild",
  appName: "5 Wild",
  // Must match vite.config.ts build.outDir; guarded by test/build-config.test.ts.
  webDir: "dist",
  android: {
    // Matches --bg in src/style.css so there is no white flash on launch.
    backgroundColor: "#0e0f13",
  },
  server: {
    androidScheme: "https",
    // For live reload on a physical device, uncomment and point at your LAN IP
    // while `npm run dev` is running, then re-run `npx cap sync android`:
    // url: "http://192.168.1.x:5173",
    // cleartext: true,
  },
}

export default config
