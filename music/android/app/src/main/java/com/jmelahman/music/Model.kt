package com.jmelahman.music

import android.app.Application
import android.content.ComponentName
import android.content.Intent
import android.net.Uri
import android.os.SystemClock
import androidx.compose.runtime.derivedStateOf
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
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import kotlin.random.Random

// The screens, by name rather than by object, so a reloaded library
// still resolves them. Home and Library are the roots the bottom bar
// switches between; the rest stack on top.
sealed class Screen {
    data object Home : Screen()
    data class Library(val view: Shelf = Shelf.Artists) : Screen()
    data class ArtistPage(val artist: String) : Screen()
    data class AlbumPage(val artist: String, val album: String) : Screen()
    data object Search : Screen()
    data class PlaylistPage(val id: String) : Screen()
    data object LibraryMix : Screen()
}

// The library's three views.
enum class Shelf { Artists, Albums, Playlists }

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

    // Whether the supermix's last read failed; the card offers another.
    var mixFailed by mutableStateOf(false)
        private set

    // Why the account's own mix last failed, such as expired cookies,
    // when the supermix fell back to the radio.
    var accountFailed by mutableStateOf("")
        private set

    // Where the supermix came from: "account", the YouTube Music
    // account's own, or "radio", seeded from the library.
    var mixSource by mutableStateOf("")
        private set

    // How many tracks each mix holds (Settings.MIX_SIZES).
    var mixSize by mutableStateOf(settings.mixSize)
        private set

    // Whether the server has YouTube Music set up; the supermix is
    // offered only then.
    var supermixReady by mutableStateOf(false)
        private set

    // The day's mix of owned tracks.
    var libraryMix by mutableStateOf<List<Owned>?>(null)
        private set

    // Which of the day's other library mixes is showing; 0 is its own,
    // the one the phone keeps.
    private var libRemix = 0

    // The server, empty until the first launch is told one.
    var server by mutableStateOf(settings.baseUrl)
        private set
    var connecting by mutableStateOf(false)
        private set

    var playlists by mutableStateOf<List<Playlist>?>(null)
        private set

    // Each playlist opened, with its tracks, by id.
    var opened by mutableStateOf<Map<String, Playlist>>(emptyMap())
        private set

    // What is kept out of the mixes: paths, and yt:<id> for streams.
    var excluded by mutableStateOf<Set<String>>(emptySet())
        private set

    // What the phone keeps in step with the server; see Settings.kept.
    // The library shows only what is on the phone, and what that is.
    var downloadedOnly by mutableStateOf(false)
    var onPhone by mutableStateOf<Set<String>>(emptySet())
        private set

    fun refreshOnPhone() {
        viewModelScope.launch { onPhone = withContext(Dispatchers.IO) { Downloads.onPhone(context) } }
    }

    var kept by mutableStateOf(settings.kept)
        private set

    var buys by mutableStateOf<List<Buy>>(emptyList())
        private set

    // Whether the server has a store to buy from.
    var buysReady by mutableStateOf(false)
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

    // The phone's own player, as it last reported. What the screens read
    // (playing, isPlaying and the rest) is this, or the web player's when
    // the phone drives one.
    private var phonePlaying by mutableStateOf<NowPlaying?>(null)
    private var phoneIsPlaying by mutableStateOf(false)
    private var phoneDuration by mutableStateOf(0L)
    private var phoneHasNext by mutableStateOf(false)
    private var phoneShuffle by mutableStateOf(false)
    private var phoneRepeat by mutableStateOf(Player.REPEAT_MODE_OFF)

    val playing: NowPlaying?
        get() = if (remoteId == null) phonePlaying else remoteState?.let { NowPlaying(it.key, it.title, it.subtitle, it.art) }
    val isPlaying: Boolean get() = if (remoteId == null) phoneIsPlaying else remoteState?.playing == true
    val duration: Long get() = if (remoteId == null) phoneDuration else ((remoteState?.duration ?: 0.0) * 1000).toLong()
    val hasNext: Boolean get() = if (remoteId == null) phoneHasNext else remoteState?.hasNext == true
    val shuffle: Boolean get() = if (remoteId == null) phoneShuffle else remoteState?.shuffle == true
    val repeat: Int
        get() = if (remoteId == null) {
            phoneRepeat
        } else {
            when (remoteState?.repeat) {
                "all" -> Player.REPEAT_MODE_ALL
                "one" -> Player.REPEAT_MODE_ONE
                else -> Player.REPEAT_MODE_OFF
            }
        }
    var expanded by mutableStateOf(false)

    // The streams in the queue, by media id, so the player can offer to
    // buy the one it is on.
    private var streams: Map<String, YTTrack> = emptyMap()
    val playingStream: YTTrack? get() = if (remoteId == null) playing?.let { streams[it.id] } else null

    // ------------------------------------------------------------ remote

    // The web players open on the server, while the app is in front.
    var remotePlayers by mutableStateOf<List<RemotePlayer>>(emptyList())
        private set

    // The web player the phone drives, or null for the phone's own.
    var remoteId by mutableStateOf<String?>(null)
        private set
    val remote: RemotePlayer? get() = remoteId?.let { id -> remotePlayers.find { it.id == id } }
    private val remoteState: RemoteState? get() = remote?.state

    // When the web player's state came, so its position can run on
    // between reports.
    private var remoteAt = 0L
    private var remoteWatch: Job? = null

    // watchRemote keeps the server's event stream open while on, coming
    // back after a drop.
    fun watchRemote(on: Boolean) {
        remoteWatch?.cancel()
        remoteWatch = null
        if (!on) return
        remoteWatch = viewModelScope.launch(Dispatchers.IO) {
            var wait = 1_000L
            while (isActive) {
                if (Api.baseUrl.isNotEmpty()) {
                    val conn = Api.remoteEvents()
                    // A blocked read only ends when its connection does.
                    val hang = coroutineContext[Job]!!.invokeOnCompletion { conn.disconnect() }
                    try {
                        Api.readRemote(conn) { players ->
                            wait = 1_000L
                            viewModelScope.launch(Dispatchers.Main) { heard(players) }
                        }
                    } catch (_: Exception) {
                        // Dropped, or the server is away; try again.
                    } finally {
                        hang.dispose()
                        conn.disconnect()
                    }
                }
                delay(wait)
                wait = (wait * 2).coerceAtMost(30_000L)
            }
        }
    }

    private fun heard(players: List<RemotePlayer>) {
        val was = remoteState
        remotePlayers = players
        remoteAt = SystemClock.elapsedRealtime()
        seeks++
        val id = remoteId ?: return
        val now = players.find { it.id == id }
        if (now == null) {
            remoteId = null
            notice("The web player closed; back to this phone")
        } else if (now.state?.blocked == true && was?.blocked != true) {
            notice("The browser wants a click on its page before it plays")
        }
    }

    // useRemote has the phone drive a web player, or itself with null;
    // the phone's own player stops while it drives another.
    fun useRemote(id: String?) {
        if (id != null) controller?.pause()
        remoteId = id
        seeks++
    }

    private fun command(what: String, build: JSONObject.() -> Unit = {}) {
        val id = remoteId ?: return
        val cmd = JSONObject().put("do", what).apply(build)
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.remote(id, cmd) }
            } catch (e: Exception) {
                notice("The web player didn't answer: ${e.message}")
            }
        }
    }

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
        phonePlaying = item?.let {
            val m = it.mediaMetadata
            NowPlaying(
                id = it.mediaId,
                title = m.title?.toString().orEmpty(),
                subtitle = listOfNotNull(m.artist, m.albumTitle).joinToString(" · "),
                art = m.artworkUri?.toString(),
            )
        }
        phoneIsPlaying = p.isPlaying
        phoneDuration = p.duration.coerceAtLeast(0)
        phoneHasNext = p.hasNextMediaItem()
        phoneShuffle = p.shuffleModeEnabled
        phoneRepeat = p.repeatMode
        if (item == null && remoteId == null) expanded = false
    }

    fun position(): Long {
        if (remoteId == null) return controller?.currentPosition ?: 0
        val s = remoteState ?: return 0
        val ran = if (s.playing) SystemClock.elapsedRealtime() - remoteAt else 0
        val at = (s.position * 1000).toLong() + ran
        return if (s.duration > 0) at.coerceAtMost((s.duration * 1000).toLong()) else at
    }

    fun togglePlay() {
        if (remoteId != null) return command("toggle")
        val c = controller ?: return
        if (c.isPlaying) c.pause() else c.play()
    }

    fun next() {
        if (remoteId != null) command("next") else controller?.seekToNext()
    }

    fun previous() {
        if (remoteId != null) command("prev") else controller?.seekToPrevious()
    }

    // jump plays a web player's queue from one of its tracks.
    fun jump(index: Int) = command("jump") { put("i", index) }

    // seeks counts the jumps, so a paused rail reads the new position.
    var seeks by mutableStateOf(0)
        private set

    fun seekTo(ms: Long) {
        if (remoteId != null) return command("seek") { put("at", ms / 1000.0) }
        controller?.seekTo(ms) ?: return
        seeks++
    }

    fun toggleShuffle() {
        if (remoteId != null) return command("shuffle") { put("on", !shuffle) }
        val c = controller ?: return
        c.shuffleModeEnabled = !c.shuffleModeEnabled
    }

    // cycleRepeat goes from off to the whole queue to the one track.
    fun cycleRepeat() {
        if (remoteId != null) {
            val next = mapOf("off" to "all", "all" to "one").getOrDefault(remoteState?.repeat ?: "off", "off")
            return command("repeat") { put("mode", next) }
        }
        val c = controller ?: return
        c.repeatMode = when (c.repeatMode) {
            Player.REPEAT_MODE_OFF -> Player.REPEAT_MODE_ALL
            Player.REPEAT_MODE_ALL -> Player.REPEAT_MODE_ONE
            else -> Player.REPEAT_MODE_OFF
        }
    }

    init {
        Api.baseUrl = settings.baseUrl
        if (server.isNotEmpty()) start()
    }

    // start reads everything from the server and has the phone catch up
    // with what it keeps.
    private fun start() {
        reload()
        checkForUpdate()
        viewModelScope.launch { refreshBuys() }
        Sync.schedule(context, settings.syncOvernight)
        Sync.now(context)
    }

    // connect checks that an address is one of these servers before the
    // app settles on it.
    fun connect(url: String) {
        val base = url.trim().trimEnd('/').let { if ("://" in it) it else "http://$it" }
        if (connecting) return
        connecting = true
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.health(base) }
                settings.baseUrl = base
                server = base
                start()
            } catch (e: Exception) {
                notice("No music server at $base: ${e.message}")
            }
            connecting = false
        }
    }

    override fun onCleared() {
        controller?.removeListener(listener)
        MediaController.releaseFuture(controllerFuture)
    }

    // ------------------------------------------------------------ library

    fun reload() {
        loadError = null
        loadPlaylists()
        loadLibraryMix()
        viewModelScope.launch {
            runCatching { excluded = withContext(Dispatchers.IO) { Api.excluded() } }
        }
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
        // The library is a root of its own; back from it is home.
        if (stack.size <= 1) {
            if (stack.first() == Screen.Home) return false
            stack = listOf(Screen.Home)
            return true
        }
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

    // loadMix reads the supermix. There is no remixing it: YouTube Music
    // makes it, and asking again brings the same.
    fun loadMix() {
        if (mixing) return
        mixing = true
        mixFailed = false
        viewModelScope.launch {
            try {
                val (m, source) = withContext(Dispatchers.IO) { Api.ytSupermix() }
                mix = m
                mixSource = source
            } catch (e: Exception) {
                mixFailed = true
                notice("The mix could not be built: ${e.message}")
            }
            mixing = false
            accountFailed = runCatching { withContext(Dispatchers.IO) { Api.ytAccountFailed() } }.getOrDefault("")
        }
    }

    // backdrop is the cover whose colour washes the top of the screen:
    // the album's or artist's in view, a mix's or playlist's first, or else
    // what is playing.
    fun backdrop(): String? = when (val s = screen) {
        is Screen.AlbumPage -> Api.coverUrl(s.artist, s.album)
        is Screen.ArtistPage ->
            library?.find { it.name == s.artist }?.albums?.firstOrNull()?.let { Api.coverUrl(s.artist, it.name) }
        Screen.LibraryMix -> libraryMix?.firstOrNull()?.let { Api.coverUrl(it.artist, it.album) }
        is Screen.PlaylistPage -> opened[s.id]?.tracks?.firstOrNull()?.let { Api.coverUrl(it.artist, it.album) }
        else -> null
    } ?: playing?.art

    // ---------------------------------------------------------------- play

    private fun streamingAllowed() = settings.allowMetered || !Settings.metered(context)

    // playStreams plays YouTube tracks through the server's proxy, under
    // the same network rules as streaming the library.
    fun playStreams(tracks: List<YTTrack>, index: Int) {
        if (remoteId != null) {
            val list = JSONArray()
            for (t in tracks) {
                list.put(JSONObject().put("videoId", t.videoId).put("title", t.title).put("artist", t.artist).put("thumb", t.thumb))
            }
            return command("streams") { put("tracks", list).put("index", index) }
        }
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
        play(items, index) { streams = tracks.associateBy { "yt:${it.videoId}" } }
    }

    // play starts a list of owned tracks: saved ones play from the phone,
    // the rest stream, and a metered network only ever carries what the
    // user allowed.
    // owners holds each track's artist and album, which a track alone
    // doesn't carry.
    // remember says to keep the album among the ones last played.
    fun play(items: List<Owned>, index: Int, shuffle: Boolean = false, remember: Boolean = false) {
        if (items.isEmpty()) return
        val order = if (shuffle) items.indices.shuffled() else items.indices.toList()
        val first = if (shuffle) 0 else index
        if (remoteId != null) {
            // The web player plays them from the server, saved or not.
            val list = JSONArray()
            for (i in order) {
                val o = items[i]
                list.put(
                    JSONObject().put("path", o.track.path).put("name", o.track.name).put("size", o.track.size)
                        .put("artist", o.artist).put("album", o.album),
                )
            }
            command("owned") { put("tracks", list).put("index", first) }
            if (remember) rememberAlbum(items[order[first]].artist, items[order[first]].album)
            return
        }
        viewModelScope.launch {
            // Which tracks are saved is a disk read each, so it is done
            // off the main thread, a whole artist at a time.
            val (saved, mediaItems) = withContext(Dispatchers.IO) {
                Downloads.saved(context, items[order[first]].track) to order.map { i -> items[i].mediaItem() }
            }
            if (!saved && !streamingAllowed()) {
                notice("Mobile data — streaming is off in settings, and this track isn't saved")
                return@launch
            }
            play(mediaItems, first) { streams = emptyMap() }
            if (remember) rememberAlbum(items[order[first]].artist, items[order[first]].album)
        }
    }

    fun playAlbum(artist: Artist, album: Album, index: Int = 0, shuffle: Boolean = false) =
        play(album.tracks.map { Owned(it, artist.name, album.name) }, index, shuffle, remember = true)

    fun playArtist(artist: Artist, shuffle: Boolean) =
        play(artist.albums.flatMap { al -> al.tracks.map { Owned(it, artist.name, al.name) } }, 0, shuffle, remember = true)

    private fun play(items: List<MediaItem>, index: Int, then: () -> Unit) {
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

    private fun Owned.mediaItem(): MediaItem {
        // A saved track plays from the phone; anything else streams.
        val local = Downloads.fileFor(context, track.path)?.takeIf { it.length() > 0 }
        val uri = local?.let(Uri::fromFile) ?: Uri.parse(Api.audioUrl(track.path))
        return MediaItem.Builder()
            .setMediaId(track.path)
            .setUri(uri)
            .setMediaMetadata(
                MediaMetadata.Builder()
                    .setTitle(track.title)
                    .setArtist(artist)
                    .setAlbumTitle(album)
                    .setArtworkUri(Uri.parse(Api.coverUrl(artist, album)))
                    .build(),
            )
            .build()
    }

    // rememberAlbum keeps the albums last played, newest first, for the
    // home page's "Jump back in".
    private fun rememberAlbum(artist: String, album: String) {
        val key = artist to album
        // From the saved list, which a sync may have renamed albums in.
        recent = (listOf(key) + settings.recent.filter { it != key }).take(12)
        settings.recent = recent
    }

    // ----------------------------------------------------------- offline

    // keep has the phone keep something (Settings.kept) or let it go; the
    // sync then fetches or deletes its tracks, on Wi-Fi unless downloads
    // on mobile data are on.
    fun keep(key: String, on: Boolean, name: String) {
        // From the saved set, which a sync may have changed since.
        kept = if (on) settings.kept + key else settings.kept - key
        settings.kept = kept
        Sync.now(context)
        notice(
            when {
                !on -> "$name will leave the phone"
                !settings.allowDownloadsMetered && Settings.metered(context) -> "Keeping $name once on Wi-Fi"
                else -> "Keeping $name on the phone"
            },
        )
    }

    fun saveAlbum(artist: Artist, album: Album) = keep(Settings.albumKey(artist.name, album.name), true, album.name)

    fun removeAlbum(artist: Artist, album: Album) = keep(Settings.albumKey(artist.name, album.name), false, album.name)

    // keepArtist keeps all of an artist's albums, and any they gain. Letting
    // the artist go lets their albums kept one by one go too.
    fun keepArtist(artist: Artist, on: Boolean) {
        if (!on) {
            settings.kept = settings.kept - artist.albums.map { Settings.albumKey(artist.name, it.name) }.toSet()
        }
        keep(Settings.artistKey(artist.name), on, artist.name)
    }

    // ------------------------------------------------------------- mixes

    // loadLibraryMix reads the library mix, and the supermix once the
    // server has one; the job is the library mix's.
    fun loadLibraryMix(): Job {
        viewModelScope.launch {
            runCatching { supermixReady = withContext(Dispatchers.IO) { Api.supermixReady() } }
            if (supermixReady && mix == null) loadMix()
        }
        return viewModelScope.launch {
            runCatching { libraryMix = withContext(Dispatchers.IO) { Api.libraryMix(mixSize, libRemix) } }
        }
    }

    // remixLibrary draws another of the day's library mixes and plays it.
    // The phone keeps the day's own.
    fun remixLibrary() {
        libRemix = 1 + Random.nextInt(1_000_000_000)
        libraryMix = null
        viewModelScope.launch {
            loadLibraryMix().join()
            libraryMix?.takeIf { it.isNotEmpty() }?.let { play(it, 0) }
        }
    }

    // sizeLibraryMix sets how long the library mix is, rereading it and
    // syncing the phone's copy when it is kept.
    fun sizeLibraryMix(n: Int) {
        if (n == mixSize) return
        mixSize = n
        settings.mixSize = n
        libraryMix = null
        loadLibraryMix()
        if (Settings.LIBRARY_MIX in kept) Sync.now(context)
    }

    // exclude keeps a track (a path, or yt:<id>) out of the mixes, or lets
    // it back in.
    fun exclude(key: String, out: Boolean) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.setExcluded(key, out) }
                excluded = if (out) excluded + key else excluded - key
                notice(if (out) "Excluded from mixing" else "Included in mixing again")
                if (out) {
                    mix = mix?.filter { "yt:${it.videoId}" != key }
                    libraryMix = libraryMix?.filter { it.track.path != key }
                }
            } catch (e: Exception) {
                notice("That could not be changed: ${e.message}")
            }
        }
    }

    // ---------------------------------------------------------- playlists

    fun loadPlaylists() {
        viewModelScope.launch {
            runCatching { playlists = withContext(Dispatchers.IO) { Api.playlists() } }
        }
    }

    fun loadPlaylist(id: String) {
        viewModelScope.launch {
            try {
                val p = withContext(Dispatchers.IO) { Api.playlist(id) }
                opened = opened + (id to p)
            } catch (e: Exception) {
                notice("The playlist could not be read: ${e.message}")
            }
        }
    }

    // savePlaylist creates a playlist (id null) or replaces one, then
    // shows it.
    fun savePlaylist(id: String?, name: String, paths: List<String>, smart: Smart?, then: (Playlist) -> Unit = {}) {
        viewModelScope.launch {
            try {
                val p = withContext(Dispatchers.IO) { Api.savePlaylist(id, name, paths, smart) }
                opened = opened + (p.id to p)
                loadPlaylists()
                if (Settings.playlistKey(p.id) in kept) Sync.now(context)
                then(p)
            } catch (e: Exception) {
                notice("The playlist could not be saved: ${e.message}")
            }
        }
    }

    fun createPlaylist(name: String, paths: List<String>, smart: Smart?) =
        savePlaylist(null, name, paths, smart) { p ->
            if (paths.isEmpty()) go(Screen.PlaylistPage(p.id)) else notice("Added to ${p.name}")
        }

    fun addToPlaylist(p: Playlist, paths: List<String>) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.addToPlaylist(p.id, paths) }
                notice("Added to ${p.name}")
                loadPlaylists()
                if (p.id in opened) loadPlaylist(p.id)
                if (Settings.playlistKey(p.id) in kept) Sync.now(context)
            } catch (e: Exception) {
                notice("That could not be added: ${e.message}")
            }
        }
    }

    fun removeFromPlaylist(p: Playlist, path: String) =
        savePlaylist(p.id, p.name, p.paths - path, null) { notice("Removed from ${it.name}") }

    fun deletePlaylist(p: Playlist) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { Api.deletePlaylist(p.id) }
                opened = opened - p.id
                if (Settings.playlistKey(p.id) in kept) keep(Settings.playlistKey(p.id), false, p.name)
                back()
                loadPlaylists()
            } catch (e: Exception) {
                notice("The playlist could not be deleted: ${e.message}")
            }
        }
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

    // failedBuys is how many buys need a hand.
    val failedBuys get() = buys.count { it.status == "failed" }

    // openPurchases shows the web app's purchases page, where failed buys
    // are retried or matched to a store link found by hand.
    fun openPurchases() {
        context.startActivity(
            Intent(Intent.ACTION_VIEW, Uri.parse("$server/#/buys")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
        )
    }

    // owns says whether the library has a stream's track, however it got
    // there: by artist and title as letters and digits alone, so
    // "TENDER — go steady" is "Tender/…/05 - Go Steady".
    fun owns(track: YTTrack): Boolean = ownKey(track.artist, track.title) in ownedKeys

    private val ownedKeys by derivedStateOf {
        library.orEmpty().flatMap { a -> a.albums.flatMap { al -> al.tracks.map { ownKey(a.name, it.title) } } }.toHashSet()
    }

    private fun ownKey(artist: String, title: String) = "${fold(artist)}\u0000${fold(title)}"

    private fun fold(s: String) = s.replace("&", "and").lowercase().filter { it.isLetterOrDigit() }

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
        buysReady = true
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

    fun saveSettings(url: String, allowMetered: Boolean, allowDownloadsMetered: Boolean, syncOvernight: Boolean) {
        val base = url.trim().trimEnd('/')
        if (!base.startsWith("http://") && !base.startsWith("https://")) {
            notice("The server needs an http:// or https:// address")
            return
        }
        val moved = base != settings.baseUrl
        settings.baseUrl = base
        server = base
        settings.allowMetered = allowMetered
        settings.allowDownloadsMetered = allowDownloadsMetered
        if (syncOvernight != settings.syncOvernight) {
            settings.syncOvernight = syncOvernight
            if (!moved) Sync.schedule(context, syncOvernight, changed = true)
        }
        if (moved) {
            library = null
            playlists = null
            opened = emptyMap()
            start()
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
