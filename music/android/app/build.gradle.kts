plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.jmelahman.music"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.jmelahman.music"
        minSdk = 29
        targetSdk = 35
        versionCode = 3
        versionName = "0.3"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
}

dependencies {
    // The player, and the session that keeps it alive in the background
    // with lock-screen controls.
    implementation("androidx.media3:media3-exoplayer:1.5.1")
    implementation("androidx.media3:media3-session:1.5.1")

    // Downloads with the OS enforcing the network they may run on.
    implementation("androidx.work:work-runtime-ktx:2.10.0")

    implementation("androidx.activity:activity-compose:1.9.3")
    implementation(platform("androidx.compose:compose-bom:2024.10.01"))
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")
}

// The server reads this sidecar to tell installed clients which version
// the downloadable APK is.
tasks.register("writeVersion") {
    doLast {
        layout.buildDirectory
            .file("outputs/apk/debug/app-debug.apk.version")
            .get().asFile
            .writeText(android.defaultConfig.versionName ?: "?")
    }
}
tasks.matching { it.name == "assembleDebug" }.configureEach {
    finalizedBy("writeVersion")
}
