// The music app's Android client. The build borrows Google's tooling, as
// every Android build does; the app's own code keeps to the platform:
// HttpURLConnection and org.json for the client, nothing else.
plugins {
    id("com.android.application") version "8.7.3" apply false
    id("org.jetbrains.kotlin.android") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.21" apply false
}
