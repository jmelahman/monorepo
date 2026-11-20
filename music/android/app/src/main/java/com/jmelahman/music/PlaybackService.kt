package com.jmelahman.music

import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

// The player as a foreground service: it outlives the activity, Android
// treats it like any other music app, and the lock screen gets its
// controls through the Media3 session.
class PlaybackService : MediaSessionService() {
    private var session: MediaSession? = null
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var watch: Job? = null

    // Whether the item playing has been counted, so a play counts once.
    private var counted = false

    // A play counts toward the server's play counts, which the smart
    // playlists and the library mix read, once half the track or 30
    // seconds of it has played, as the webapp counts. Streams aren't
    // owned and don't count.
    private val counter = object : Player.Listener {
        override fun onMediaItemTransition(item: MediaItem?, reason: Int) {
            counted = false
        }

        override fun onIsPlayingChanged(isPlaying: Boolean) {
            watch?.cancel()
            if (!isPlaying) return
            watch = scope.launch {
                while (isActive) {
                    check()
                    delay(5000)
                }
            }
        }
    }

    private fun check() {
        val player = session?.player ?: return
        val path = player.currentMediaItem?.mediaId ?: return
        if (counted || path.startsWith("yt:") || player.duration <= 0) return
        if (player.currentPosition < minOf(30_000L, player.duration / 2)) return
        counted = true
        scope.launch(Dispatchers.IO) { runCatching { Api.played(path) } }
    }

    override fun onCreate() {
        super.onCreate()
        val player = ExoPlayer.Builder(this)
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(C.USAGE_MEDIA)
                    .setContentType(C.AUDIO_CONTENT_TYPE_MUSIC)
                    .build(),
                /* handleAudioFocus = */ true,
            )
            .setHandleAudioBecomingNoisy(true)
            .build()
        if (Api.baseUrl.isEmpty()) Api.baseUrl = Settings(this).baseUrl
        player.addListener(counter)
        session = MediaSession.Builder(this, player).build()
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession? = session

    // Swiping the app away stops a paused player with it; one that is
    // playing carries on, as music apps do.
    override fun onTaskRemoved(rootIntent: android.content.Intent?) {
        val player = session?.player
        if (player == null || !player.playWhenReady || player.mediaItemCount == 0) stopSelf()
    }

    override fun onDestroy() {
        scope.cancel()
        session?.run {
            player.release()
            release()
        }
        session = null
        super.onDestroy()
    }
}
