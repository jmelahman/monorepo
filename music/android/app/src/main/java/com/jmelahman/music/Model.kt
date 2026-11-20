package com.jmelahman.music

import android.app.Application
import android.content.ComponentName
import android.content.Intent
import android.net.Uri
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.Player
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import androidx.work.Constraints
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.workDataOf
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

// The screens, by name rather than by object, so a reloaded library
// still resolves them. Home and Library are the roots the bottom bar
// switches between; the rest stack on top.
sealed class Screen {
    data object Home : Screen()
    data class Library(val artists: Boolean = false) : Screen()
    data class ArtistPage(val artist: String) : Screen()
    data class AlbumPage(val artist: String, val album: String) : Screen()
    data object Search : Screen()
}

// What the player is on, as the screens draw it.
data class NowPlaying(
    val id: String,
    val title: String,
    val subtitle: String,
    val art: String?,
)

// The app's state and everything it does, outside the activity, so a
// rotation or a trip to the settings keeps the library, the search and
// the session as they were.
class Model(app: Application) : AndroidViewModel(app) {
    private val context get() = getApplication<Application>()
    val settings = Settings(app)

    var library by mutableStateOf<List<Artist>?>(null)
        private set
    var loadError by mutableStateOf<String?>(null)
        private set
    var stack by mutableStateOf<List<Screen>>(listOf(Screen.Home))
        private set

    var query by mutableStateOf("")
    var results by mutableStateOf<List<YTTrack>?>(null)
        private set
    var searching by mutableStateOf(false)
        private set
    var mix by mutableStateOf<List<YTTrack>?>(null)
        private set
    var mixing by mutableStateOf(false)
        private set

    var buys by mutableStateOf<List<Buy>>(emptyList())
        private set
    private var buysPoll: Job? = null

    var update by mutableStateOf<AppUpdate?>(null)
        private set
    var updating by mutableStateOf(false)
        private set
    var recent by mutableStateOf(settings.recent)
        private set

    // One-off messages, shown as a snackbar.
    private val notices = Channel<String>(Channel.BUFFERED)
    val noticeFlow = notices.receiveAsFlow()
    fun notice(text: String) {
        notices.trySend(text)
    }

    // ------------------------------------------------------------ player

    var controller by mutableStateOf<MediaController?>(null)
        private set
    var playing by mutableStateOf<NowPlaying?>(null)
        private set
    var isPlaying by mutableStateOf(false)
        private set
    var duration by mutableStateOf(0L)
        private set
    var hasNext by mutableStateOf(false)
        private set
    var expanded by mutableStateOf(false)

    // The streams in the queue, by media id, so the player can offer to
    // buy the one it is on.
    private var streams: Map<String, YTTrack> = emptyMap()
    val playingStream: YTTrack? get() = playing?.let { streams[it.id] }

    private val listener = object : Player.Listener {
        override fun onEvents(player: Player, events: Player.Events) = sync(player)
    }

    private val controllerFuture = MediaController.Builder(
        app,
        SessionToken(app, ComponentName(app, PlaybackService::class.java)),
    ).buildAsync().also { f ->
        f.addListener({
            val c = runCatching { f.get() }.getOrNull() ?: return@addListener
            c.addListener(listener)
            controller = c
            sync(c)
        }, ContextCompat.getMainExecutor(app))
    }

    private fun sync(p: Player) {
        val item = p.currentMediaItem
        playing = item?.let {
            val m = it.mediaMetadata
            NowPlaying(
                id = it.mediaId,
                title = m.title?.toString().orEmpty(),
                subtitle = listOfNotNull(m.artist, m.albumTitle).joinToString(" · "),
                art = m.artworkUri?.toString(),
            )
        }
        isPlaying = p.isPlaying
        duration = p.duration.coerceAtLeast(0)
        hasNext = p.hasNextMediaItem()
        if (item == null) expanded = false
    }

    fun position(): Long = controller?.currentPosition ?: 0

    fun togglePlay() {
        val c = controller ?: return
        if (c.isPlaying) c.pause() else c.play()
    }

    fun next() = controller?.seekToNext()

    fun previous() = controller?.seekToPrevious()

    fun seekTo(ms: Long) = controller?.seekTo(ms)

    init {
        Api.baseUrl = settings.baseUrl
        reload()
        checkForUpdate()
        viewModelScope.launch { refreshBuys() }
    }

    override fun onCleared() {
        controller?.removeListener(listener)
        MediaController.releaseFuture(controllerFuture)
    }

    // ------------------------------------------------------------ library

    fun reload() {
        loadError = null
        viewModelScope.launch {
            try {
                library = withContext(Dispatchers.IO) { Api.library() }
            } catch (e: Exception) {
                if (library == null) loadError = "The library could not be read: ${e.message}"
                else notice("The library could not be refreshed")
            }
        }
    }

    fun artist(name: String) = library?.find { it.name == name }

    fun album(artist: String, album: String): Pair<Artist, Album>? {
        val a = artist(artist) ?: return null
        return a.albums.find { it.name == album }?.let { a to it }
    }

    // --------------------------------------------------------- navigation

    val screen get() = stack.last()

    fun go(screen: Screen) {
        expanded = false
        stack = stack + screen
    }

    // tab switches the root, leaving whatever was stacked on the old one.
    fun tab(root: Screen) {
        expanded = false
        stack = listOf(root)
    }

    fun back(): Boolean {
        if (expanded) {
            expanded = false
            return true
        }
        if (stack.size <= 1) return false
        stack = stack.dropLast(1)
        return true
    }

    // ---------------------------------------------------------- streams

    fun search(q: String) {
        val query = q.trim()
        if (query.isEmpty() || searching) return
        searching = true
        viewModelScope.launch {
            try {
                results = withContext(Dispatchers.IO) { Api.ytSearch(query) }
            } catch (e: Exception) {
                notice("The search could not be read: ${e.message}")
            }
            searching = false
        }
    }

    fun buildMix() {
        if (mixing) return
        mixing = true
        viewModelScope.launch {
            try {
                val m = withContext(Dispatchers.IO) { Api.ytSupermix() }
                mix = m
                if (m.isNotEmpty()) playStreams(m, 0)
            } catch (e: Exception) {
                notice("The mix could not be built: ${e.message}")
            }
            mixing = false
        }
    }

    // ---------------------------------------------------------------- play

    private fun streamingAllowed() = settings.allowMetered || !Settings.metered(context)

    // playStreams plays YouTube tracks through the server's proxy, under
    // the same network rules as streaming the library.
    fun playStreams(tracks: List<YTTrack>, index: Int) {
        if (!streamingAllowed()) {
            notice("Mobile data — streaming is off in settings")
            return
        }
        val items = tracks.map { t ->
            MediaItem.Builder()
                .setMediaId("yt:${t.videoId}")
                .setUri(Uri.parse(Api.streamUrl(t.videoId)))
                .setMediaMetadata(
                    MediaMetadata.Builder()
                        .setTitle(t.title)
                        .setArtist(t.artist)
                        .setArtworkUri(t.thumb.takeIf { it.isNotEmpty() }?.let(Uri::parse))
                        .build(),
                )
                .build()
        }
        start(items, index) { streams = tracks.associateBy { "yt:${it.videoId}" } }
    }

    // play starts a list of owned tracks: saved ones play from the phone,
    // the rest stream, and a metered network only ever carries what the
    // user allowed.
    // owners holds each track's artist and album, which a track alone
    // doesn't carry.
    fun play(owners: List<Pair<Artist, Album>>, items: List<Track>, index: Int, shuffle: Boolean = false) {
        val order = if (shuffle) items.indices.shuffled() else items.indices.toList()
        val first = if (shuffle) 0 else index
        val track = items[order[first]]
        if (!Downloads.saved(context, track) && !streamingAllowed()) {
            notice("Mobile data — streaming is off in settings, and this track isn't saved")
            return
        }
        val mediaItems = order.map { i -> items[i].mediaItem(owners[i].first, owners[i].second) }
        val (artist, album) = owners[order[first]]
        start(mediaItems, first) { streams = emptyMap() }
        rememberAlbum(artist, album)
    }

    fun playAlbum(artist: Artist, album: Album, index: Int = 0, shuffle: Boolean = false) =
        play(List(album.tracks.size) { artist to album }, album.tracks, index, shuffle)

    fun playArtist(artist: Artist, shuffle: Boolean) {
        val owners = artist.albums.flatMap { al -> al.tracks.map { artist to al } }
        if (owners.isEmpty()) return
        play(owners, artist.albums.flatMap { it.tracks }, 0, shuffle)
    }

    private fun start(items: List<MediaItem>, index: Int, then: () -> Unit) {
        val c = controller
        if (c == null) {
            notice("The player is still starting")
            return
        }
        then()
        c.setMediaItems(items, index, 0)
        c.prepare()
        c.play()
    }

    private fun Track.mediaItem(artist: Artist, album: Album): MediaItem {
        // A saved track plays from the phone; anything else streams.
        val local = Downloads.fileFor(context, path)?.takeIf { it.length() > 0 }
        val uri = local?.let(Uri::fromFile) ?: Uri.parse(Api.audioUrl(path))
        return MediaItem.Builder()
            .setMediaId(path)
            .setUri(uri)
            .setMediaMetadata(
                MediaMetadata.Builder()
                    .setTitle(title)
                    .setArtist(artist.name)
                    .setAlbumTitle(album.name)
                    .setArtworkUri(Uri.parse(Api.coverUrl(artist.name, album.name)))
                    .build(),
            )
            .build()
    }

    // rememberAlbum keeps the albums last played, newest first, for the
    // home page's "Jump back in".
    private fun rememberAlbum(artist: Artist, album: Album) {
        val key = artist.name to album.name
        recent = (listOf(key) + recent.filter { it != key }).take(12)
        settings.recent = recent
    }

    // ----------------------------------------------------------- offline

    // saveAlbum queues one piece of work per track, each carrying the
    // constraint the OS then enforces: unmetered networks only, unless
    // the user switched downloads on mobile data on.
    fun saveAlbum(artist: Artist, album: Album) {
        val wm = WorkManager.getInstance(context)
        val network =
            if (settings.allowDownloadsMetered) NetworkType.CONNECTED else NetworkType.UNMETERED
        for (t in album.tracks) {
            if (Downloads.saved(context, t)) continue
            val request = OneTimeWorkRequestBuilder<DownloadWorker>()
                .setInputData(workDataOf("url" to Api.audioUrl(t.path), "dest" to t.path))
                .setConstraints(Constraints.Builder().setRequiredNetworkType(network).build())
                .addTag(Downloads.TAG)
                .addTag(Downloads.albumTag(artist, album))
                .build()
            wm.enqueueUniqueWork("download:${t.path}", ExistingWorkPolicy.KEEP, request)
        }
        if (network == NetworkType.UNMETERED && Settings.metered(context)) {
            notice("Saving ${album.name} once on Wi-Fi")
        }
    }

    fun removeAlbum(artist: Artist, album: Album) {
        WorkManager.getInstance(context).cancelAllWorkByTag(Downloads.albumTag(artist, album))
        viewModelScope.launch(Dispatchers.IO) { Downloads.delete(context, album) }
        notice("Removed ${album.name} from the phone")
    }

    // --------------------------------------------------------------- buys

    // buyTrack queues a streamed track with the server's store provider,
    // then follows the queue until it lands.
    fun buyTrack(track: YTTrack) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.buy(track) }
            } catch (e: Exception) {
                notice("The buy could not be queued: ${e.message}")
                return@launch
            }
            refreshBuys()
        }
    }

    // buyFor is the newest buy of a track, if it has one.
    fun buyFor(track: YTTrack): Buy? = buys.firstOrNull {
        it.artist.equals(track.artist, ignoreCase = true) && it.title.equals(track.title, ignoreCase = true)
    }

    // refreshBuys reads the queue, says what finished since the last read,
    // and keeps polling while anything is pending.
    private suspend fun refreshBuys() {
        val next = try {
            withContext(Dispatchers.IO) { Api.buys() }
        } catch (e: Exception) {
            return
        }
        val before = buys.associate { it.id to it.status }
        var landed = false
        for (b in next) {
            val was = before[b.id] ?: continue
            if (was == b.status) continue
            when (b.status) {
                "done" -> {
                    landed = true
                    val r = b.result
                    notice(
                        when {
                            r == null -> "Bought: ${b.artist} — ${b.title}"
                            r.alreadyOwned -> "Already in the library — ${r.artist} · ${r.title}"
                            else -> "Bought: ${r.artist} — ${r.title} (${r.album})"
                        },
                    )
                }
                "failed" -> notice("The buy did not go through: ${b.artist} — ${b.title}: ${b.error}")
            }
        }
        buys = next
        if (landed) reload()
        if (buys.any { it.pending } && buysPoll?.isActive != true) {
            buysPoll = viewModelScope.launch {
                while (buys.any { it.pending }) {
                    delay(3000)
                    refreshBuys()
                }
            }
        }
    }

    // ------------------------------------------------------------ settings

    fun saveSettings(url: String, allowMetered: Boolean, allowDownloadsMetered: Boolean) {
        val base = url.trim().trimEnd('/')
        if (!base.startsWith("http://") && !base.startsWith("https://")) {
            notice("The server needs an http:// or https:// address")
            return
        }
        val moved = base != settings.baseUrl
        settings.baseUrl = base
        settings.allowMetered = allowMetered
        settings.allowDownloadsMetered = allowDownloadsMetered
        if (moved) {
            library = null
            reload()
            checkForUpdate()
        }
    }

    // ------------------------------------------------------------- update

    // checkForUpdate asks the server which build it offers; anything other
    // than this one's own version shows itself as a button.
    private fun checkForUpdate() {
        viewModelScope.launch {
            val u = withContext(Dispatchers.IO) { Api.app() }
            update = u?.takeIf { it.version.isNotEmpty() && it.version != BuildConfig.VERSION_NAME }
        }
    }

    // installUpdate fetches the newer build under the same network rules
    // as music, then hands it to the system installer, which asks for the
    // one confirmation Android always requires.
    fun installUpdate() {
        val u = update ?: return
        if (updating) return
        if (!settings.allowDownloadsMetered && Settings.metered(context)) {
            notice("Mobile data — the update waits for Wi-Fi")
            return
        }
        updating = true
        viewModelScope.launch {
            val file = File(context.cacheDir, "update/Music.apk")
            val ok = withContext(Dispatchers.IO) {
                runCatching {
                    file.parentFile?.mkdirs()
                    val conn = Api.open(Api.apkUrl())
                    try {
                        conn.inputStream.use { input -> file.outputStream().use { input.copyTo(it) } }
                    } finally {
                        conn.disconnect()
                    }
                    file.length() == u.size
                }.getOrDefault(false)
            }
            updating = false
            if (!ok) {
                notice("The update could not be downloaded")
                return@launch
            }
            val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
            context.startActivity(
                Intent(Intent.ACTION_VIEW).apply {
                    setDataAndType(uri, "application/vnd.android.package-archive")
                    addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
                },
            )
        }
    }
}
