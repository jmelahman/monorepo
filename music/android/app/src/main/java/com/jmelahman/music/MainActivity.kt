package com.jmelahman.music

import android.content.ComponentName
import android.content.Intent
import android.net.Uri
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
import androidx.compose.foundation.lazy.itemsIndexed
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
import androidx.core.content.FileProvider
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import androidx.work.Constraints
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkInfo
import androidx.work.WorkManager
import androidx.work.workDataOf
import com.google.common.util.concurrent.ListenableFuture
import java.io.File
import java.net.URL
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
class MainActivity : ComponentActivity() {    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
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
    var update by mutableStateOf<AppUpdate?>(null)
        private set
    var updating by mutableStateOf(false)
        private set
    var ytResults by mutableStateOf<List<YTTrack>?>(null)
        private set
    var ytBusy by mutableStateOf(false)
        private set

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
        checkForUpdate()
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

    fun openDiscover() {
        ytResults = null
        stack = stack + Screen.Discover
    }

    // ytSearch and ytSupermix bring the streamed half of the library: what
    // plays without being owned first.
    fun ytSearch(query: String) {
        ytBusy = true
        notice = null
        scope.launch {
            try {
                ytResults = withContext(Dispatchers.IO) { Api.ytSearch(query) }
            } catch (e: Exception) {
                notice = "The search could not be read: ${e.message}"
            }
            ytBusy = false
        }
    }

    fun ytSupermix() {
        ytBusy = true
        notice = null
        scope.launch {
            try {
                ytResults = withContext(Dispatchers.IO) { Api.ytSupermix() }
            } catch (e: Exception) {
                notice = "The mix could not be built: ${e.message}"
            }
            ytBusy = false
        }
    }

    // playStream plays YouTube tracks through the server's proxy, under
    // the same network rules as streaming the library.
    fun playStream(tracks: List<YTTrack>, index: Int) {
        val c = controller ?: return
        if (!settings.allowMetered && Settings.metered(this)) {
            notice = "Mobile data — streaming is off in settings"
            return
        }
        val items = tracks.map { track ->
            MediaItem.Builder()
                .setUri(Uri.parse(Api.streamUrl(track.videoId)))
                .setMediaMetadata(
                    MediaMetadata.Builder()
                        .setTitle(track.title)
                        .setArtist(track.artist)
                        .build(),
                )
                .build()
        }
        c.setMediaItems(items, index, 0)
        c.prepare()
        c.play()
    }

    fun back() {
        if (stack.size > 1) stack = stack.dropLast(1)
    }

    fun saveSettings(url: String, allowMetered: Boolean, allowDownloadsMetered: Boolean) {
        settings.baseUrl = url.trim().trimEnd('/')
        settings.allowMetered = allowMetered
        settings.allowDownloadsMetered = allowDownloadsMetered
        reload()
    }

    // saveAlbum queues one piece of work per track, each carrying the
    // constraint the OS then enforces: unmetered networks only, unless
    // the user switched downloads on mobile data on.
    fun saveAlbum(album: Album) {        val wm = WorkManager.getInstance(this)
        val network =
            if (settings.allowDownloadsMetered) NetworkType.CONNECTED else NetworkType.UNMETERED
        for (t in album.tracks) {
            val request = OneTimeWorkRequestBuilder<DownloadWorker>()
                .setInputData(workDataOf("url" to Api.audioUrl(t.path), "dest" to t.path))
                .setConstraints(Constraints.Builder().setRequiredNetworkType(network).build())
                .addTag(DOWNLOAD_TAG)
                .build()
            wm.enqueueUniqueWork("download:${t.path}", ExistingWorkPolicy.KEEP, request)
        }
    }

    // checkForUpdate asks the server which build it offers; anything other
    // than this one's own version shows itself as a button.
    private fun checkForUpdate() {
        scope.launch {
            val u = withContext(Dispatchers.IO) { Api.app() }
            if (u != null && u.version.isNotEmpty() && u.version != BuildConfig.VERSION_NAME) {
                update = u
            }
        }
    }

    // installUpdate fetches the newer build under the same network rules
    // as music, then hands it to the system installer, which asks for the
    // one confirmation Android always requires.
    fun installUpdate() {
        val u = update ?: return
        if (!settings.allowDownloadsMetered && Settings.metered(this)) {
            notice = "Mobile data — the update waits for Wi-Fi"
            return
        }
        updating = true
        scope.launch {
            val file = File(cacheDir, "update/Music.apk")
            val ok = withContext(Dispatchers.IO) {
                runCatching {
                    file.parentFile?.mkdirs()
                    URL(Api.baseUrl + "/api/apk").openStream().use { input ->
                        file.outputStream().use { input.copyTo(it) }
                    }
                    file.length() == u.size
                }.getOrDefault(false)
            }
            updating = false
            if (!ok) {
                notice = "The update could not be downloaded"
                return@launch
            }
            val uri = FileProvider.getUriForFile(this@MainActivity, "$packageName.fileprovider", file)
            startActivity(
                Intent(Intent.ACTION_VIEW).apply {
                    setDataAndType(uri, "application/vnd.android.package-archive")
                    addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                },
            )
        }
    }

    // play starts an album at a track: saved tracks play from the phone,
    // the rest stream, and a metered network only ever carries what the
    // user allowed.
    fun play(artist: Artist, album: Album, track: Track) {
        val c = controller ?: return
        if (!Downloads.saved(this, track) &&
            !settings.allowMetered && Settings.metered(this)
        ) {
            notice = "Mobile data — streaming is off in settings"
            return
        }
        val items = album.tracks.map { it.mediaItem(this, artist, album) }
        c.setMediaItems(items, album.tracks.indexOf(track), 0)
        c.prepare()
        c.play()
    }

    private fun Track.mediaItem(context: android.content.Context, artist: Artist, album: Album): MediaItem {
        // A saved track plays from the phone; anything else streams.
        val local = Downloads.fileFor(context, this)
        val uri =
            if (local.length() > 0) Uri.fromFile(local) else Uri.parse(Api.audioUrl(path))
        return MediaItem.Builder()
            .setUri(uri)
            .setMediaMetadata(
                MediaMetadata.Builder()
                    .setTitle(name)
                    .setArtist(artist.name)
                    .setAlbumTitle(album.name)
                    .build(),
            )
            .build()
    }
}

// The screens of a simple stack; the position is in the back of it.
sealed class Screen {
    data object Artists : Screen()
    data object Discover : Screen()
    data class Albums(val artist: Artist) : Screen()
    data class Tracks(val artist: Artist, val album: Album) : Screen()
}

// The tag every download carries, so the queue can be counted.
private const val DOWNLOAD_TAG = "music-download"

@Composable
fun MusicApp(activity: MainActivity) {
    MaterialTheme(colorScheme = darkColorScheme()) {
        Surface(modifier = Modifier.fillMaxSize()) {
            Column {
                Header(activity)
                Notice(activity.notice)
                when (val screen = activity.stack.last()) {
                    is Screen.Artists -> ArtistsScreen(activity)
                    is Screen.Discover -> DiscoverScreen(activity)
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
                is Screen.Discover -> listOf("Discover")
                is Screen.Albums -> listOf(screen.artist.name)
                is Screen.Tracks -> listOf(screen.artist.name, screen.album.name)
            }
            if (crumbs.isNotEmpty()) {
                Text(crumbs.joinToString(" / "), color = Color.Gray)
            }
        }
        Spacer(Modifier.weight(1f))
        TextButton(onClick = { activity.openDiscover() }) { Text("Discover") }
        val u = activity.update
        if (u != null) {
            TextButton(
                onClick = { activity.installUpdate() },
                enabled = !activity.updating,
            ) { Text(if (activity.updating) "Downloading…" else "Update to ${u.version}") }
        }
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

// Discover is the streamed half: search, and a supermix the server
// seeds from the owned library.
@Composable
private fun DiscoverScreen(activity: MainActivity) {
    var query by remember { mutableStateOf("") }
    Column {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                label = { Text("Search YouTube") },
                singleLine = true,
                modifier = Modifier.weight(1f),
            )
            TextButton(
                onClick = { if (query.isNotBlank()) activity.ytSearch(query.trim()) },
                enabled = !activity.ytBusy,
            ) { Text("Search") }
        }
        Row(
            modifier = Modifier.padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = { activity.ytSupermix() }, enabled = !activity.ytBusy) {
                Text("Supermix")
            }
            Spacer(Modifier.width(8.dp))
            if (activity.ytBusy) {
                Text("working…", color = Color.Gray)
            }
        }
        val results = activity.ytResults ?: return@Column
        LazyColumn {
            itemsIndexed(results) { index, track ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { activity.playStream(results, index) }
                        .padding(horizontal = 16.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(track.title, modifier = Modifier.weight(1f))
                    Text(track.artist, color = Color.Gray)
                }
            }
        }
    }
}

@Composable
private fun ArtistsScreen(activity: MainActivity) {    val artists = activity.library ?: run {
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
    var saved by remember { mutableStateOf(0) }
    var queued by remember { mutableStateOf(0) }
    LaunchedEffect(album) {
        while (true) {
            saved = Downloads.savedCount(activity, album)
            queued = withContext(Dispatchers.IO) {
                WorkManager.getInstance(activity)
                    .getWorkInfosByTag(DOWNLOAD_TAG).get()
                    .count {
                        it.state == WorkInfo.State.ENQUEUED || it.state == WorkInfo.State.RUNNING
                    }
            }
            delay(2000)
        }
    }
    Column {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = { activity.saveAlbum(album) }) { Text("Save offline") }
            Spacer(Modifier.width(8.dp))
            if (queued > 0) {
                Text("$queued queued", color = Color.Gray)
            } else {
                Text("$saved of ${album.tracks.size} saved", color = Color.Gray)
            }
        }
        LazyColumn {
            items(album.tracks) { track ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { activity.play(artist, album, track) }
                        .padding(horizontal = 16.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(track.name, modifier = Modifier.weight(1f))
                    if (Downloads.saved(activity, track)) {
                        Text("saved", color = Color.Gray)
                    }
                }
            }
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
    var allowDownloadsMetered by remember { mutableStateOf(activity.settings.allowDownloadsMetered) }

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
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Download on mobile data", modifier = Modifier.weight(1f))
                    Switch(checked = allowDownloadsMetered, onCheckedChange = { allowDownloadsMetered = it })
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    activity.saveSettings(url, allowMetered, allowDownloadsMetered)
                    onDismiss()
                },
            ) { Text("Save") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
