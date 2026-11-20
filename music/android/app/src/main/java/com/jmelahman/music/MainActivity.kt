package com.jmelahman.music

import android.content.ComponentName
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import com.google.common.util.concurrent.ListenableFuture
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

// The one screen: the library, browsed the way the server lays it out,
// played through the session service. The settings carry the native
// client's point: streaming on metered data is off unless switched on.
class MainActivity : ComponentActivity() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    lateinit var settings: Settings
        private set
    private lateinit var controllerFuture: ListenableFuture<MediaController>

    var controller by mutableStateOf<MediaController?>(null)
        private set
    var library by mutableStateOf<List<Artist>?>(null)
        private set
    var loadError by mutableStateOf<String?>(null)
        private set
    var stack by mutableStateOf<List<Screen>>(listOf(Screen.Artists))
    var notice by mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        settings = Settings(this)
        Api.baseUrl = settings.baseUrl
        enableEdgeToEdge()

        controllerFuture = MediaController.Builder(
            this,
            SessionToken(this, ComponentName(this, PlaybackService::class.java)),
        ).buildAsync()
        controllerFuture.addListener(
            { controller = controllerFuture.get() },
            ContextCompat.getMainExecutor(this),
        )

        setContent { MusicApp(this) }
        reload()
    }

    override fun onDestroy() {
        scope.cancel()
        MediaController.releaseFuture(controllerFuture)
        super.onDestroy()
    }

    fun reload() {
        library = null
        loadError = null
        notice = null
        scope.launch {
            try {
                library = withContext(Dispatchers.IO) { Api.library() }
            } catch (e: Exception) {
                loadError = "The library could not be read: ${e.message}"
            }
        }
    }

    fun open(artist: Artist) {
        stack = stack + Screen.Albums(artist)
    }

    fun open(artist: Artist, album: Album) {
        stack = stack + Screen.Tracks(artist, album)
    }

    fun back() {
        if (stack.size > 1) stack = stack.dropLast(1)
    }

    fun saveSettings(url: String, allowMetered: Boolean) {
        settings.baseUrl = url.trim().trimEnd('/')
        settings.allowMetered = allowMetered
        reload()
    }

    // play starts an album at a track, with the metered-network guard that
    // made this a native app.
    fun play(artist: Artist, album: Album, track: Track) {
        val c = controller ?: return
        if (!settings.allowMetered && Settings.metered(this)) {
            notice = "Mobile data — streaming is off in settings"
            return
        }
        val items = album.tracks.map { it.mediaItem(artist, album) }
        c.setMediaItems(items, album.tracks.indexOf(track), 0)
        c.prepare()
        c.play()
    }

    private fun Track.mediaItem(artist: Artist, album: Album) = MediaItem.Builder()
        .setUri(Api.audioUrl(path))
        .setMediaMetadata(
            MediaMetadata.Builder()
                .setTitle(name)
                .setArtist(artist.name)
                .setAlbumTitle(album.name)
                .build(),
        )
        .build()
}

// The screens of a simple stack; the position is in the back of it.
sealed class Screen {
    data object Artists : Screen()
    data class Albums(val artist: Artist) : Screen()
    data class Tracks(val artist: Artist, val album: Album) : Screen()
}

@Composable
fun MusicApp(activity: MainActivity) {
    MaterialTheme(colorScheme = darkColorScheme()) {
        Surface(modifier = Modifier.fillMaxSize()) {
            Column {
                Header(activity)
                Notice(activity.notice)
                when (val screen = activity.stack.last()) {
                    is Screen.Artists -> ArtistsScreen(activity)
                    is Screen.Albums -> AlbumsScreen(activity, screen.artist)
                    is Screen.Tracks -> TracksScreen(activity, screen.artist, screen.album)
                }
                NowPlaying(activity)
            }
            BackHandler(enabled = activity.stack.size > 1) { activity.back() }
        }
    }
}

@Composable
private fun Header(activity: MainActivity) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("Music", fontWeight = FontWeight.Bold)
        Spacer(Modifier.width(8.dp))
        activity.stack.lastOrNull()?.let { screen ->
            val crumbs = when (screen) {
                is Screen.Artists -> emptyList()
                is Screen.Albums -> listOf(screen.artist.name)
                is Screen.Tracks -> listOf(screen.artist.name, screen.album.name)
            }
            if (crumbs.isNotEmpty()) {
                Text(crumbs.joinToString(" / "), color = Color.Gray)
            }
        }
        Spacer(Modifier.weight(1f))
        var showSettings by remember { mutableStateOf(false) }
        TextButton(onClick = { showSettings = true }) { Text("Settings") }
        if (showSettings) {
            SettingsDialog(activity) { showSettings = false }
        }
    }
}

@Composable
private fun Notice(text: String?) {
    if (text != null) {
        Text(
            text,
            color = MaterialTheme.colorScheme.error,
            modifier = Modifier.padding(horizontal = 16.dp),
        )
    }
}

@Composable
private fun ArtistsScreen(activity: MainActivity) {
    val artists = activity.library ?: run {
        LoadingOrError(activity)
        return
    }
    LazyColumn {
        items(artists) { artist ->
            ListRow(artist.name) { activity.open(artist) }
        }
    }
}

@Composable
private fun AlbumsScreen(activity: MainActivity, artist: Artist) {
    LazyColumn {
        items(artist.albums) { album ->
            ListRow("${album.name} (${album.tracks.size})") { activity.open(artist, album) }
        }
    }
}

@Composable
private fun TracksScreen(activity: MainActivity, artist: Artist, album: Album) {
    LazyColumn {
        items(album.tracks) { track ->
            ListRow(track.name) { activity.play(artist, album, track) }
        }
    }
}

@Composable
private fun ListRow(text: String, onClick: () -> Unit) {
    Text(
        text,
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 14.dp),
    )
}

@Composable
private fun LoadingOrError(activity: MainActivity) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        val error = activity.loadError
        if (error != null) {
            Text(error, color = MaterialTheme.colorScheme.error)
            TextButton(onClick = { activity.reload() }) { Text("Retry") }
        } else {
            Text("Loading…")
        }
    }
}

// NowPlaying polls the session lightly; the service itself is what keeps
// playing when the screen and the activity are gone.
@Composable
private fun NowPlaying(activity: MainActivity) {
    val c = activity.controller ?: return
    var playing by remember { mutableStateOf(false) }
    var title by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(c) {
        while (true) {
            playing = c.isPlaying
            title = c.currentMediaItem?.mediaMetadata?.title?.toString()
            delay(500)
        }
    }
    if (title == null) return

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title ?: "",
            modifier = Modifier.weight(1f),
            color = MaterialTheme.colorScheme.primary,
        )
        TextButton(
            onClick = {
                if (playing) c.pause() else c.play()
            },
        ) { Text(if (playing) "Pause" else "Play") }
    }
}

@Composable
private fun SettingsDialog(activity: MainActivity, onDismiss: () -> Unit) {
    var url by remember { mutableStateOf(activity.settings.baseUrl) }
    var allowMetered by remember { mutableStateOf(activity.settings.allowMetered) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Settings") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text("Server") },
                    singleLine = true,
                )
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Stream on mobile data", modifier = Modifier.weight(1f))
                    Switch(checked = allowMetered, onCheckedChange = { allowMetered = it })
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    activity.saveSettings(url, allowMetered)
                    onDismiss()
                },
            ) { Text("Save") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
