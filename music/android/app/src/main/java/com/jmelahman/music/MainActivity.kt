package com.jmelahman.music

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels

// The activity only hosts the UI; the state lives in the Model, so it
// survives rotation, and the player in the PlaybackService, so it
// survives the activity.
class MainActivity : ComponentActivity() {
    private val model: Model by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
        )
        super.onCreate(savedInstanceState)
        setContent { MusicApp(model) }
    }
}
