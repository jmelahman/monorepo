package com.jmelahman.music

import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

// The library as the server lists it, one pass of org.json over the
// api/ the webapp already speaks. HttpURLConnection everywhere; the app
// keeps to the platform's own HTTP and JSON.
data class Track(
    val name: String,
    val path: String,
    val size: Long,
    val plays: Int = 0,
    val excluded: Boolean = false,
    // How long it plays, in seconds; 0 when the server couldn't tell.
    val duration: Double = 0.0,
) {
    // The name without the leading track number a file name carries.
    val title: String get() = name.replace(TRACK_NUMBER, "").ifEmpty { name }
}

data class Album(val name: String, val tracks: List<Track>) {
    val size: Long get() = tracks.sumOf { it.size }
}

data class Artist(val name: String, val albums: List<Album>)

private val TRACK_NUMBER = Regex("""^\d+[\s._-]*""")

// A track with the artist and album it belongs to, as the playlists and
// the library mix list them.
data class Owned(val track: Track, val artist: String, val album: String)

// A mix made for you from the library's labels: a genre's, a mood's, or
// a mood's within a genre. Covers are artist and album pairs.
data class MadeMix(
    val id: String,
    val title: String,
    val tracks: Int,
    val covers: List<Pair<String, String>>,
    val blurb: String,
)

// A smart playlist's rules: tracks that meet all, or any, of them, in an
// order, up to a limit (0 for none).
data class Rule(val field: String, val op: String, val value: String)

data class Smart(
    val match: String = "all",
    val rules: List<Rule> = emptyList(),
    val sort: String = "artist",
    val limit: Int = 0,
)

// A playlist: chosen tracks, or rules that choose them. tracks is only
// there when one playlist was asked for.
data class Playlist(
    val id: String,
    val name: String,
    val paths: List<String>,
    val smart: Smart?,
    val count: Int,
    val tracks: List<Owned>?,
)

// The Android client's build that the server offers, when it is newer
// than the one asking.
data class AppUpdate(val version: String, val size: Long)

// A playable YouTube track, streamed through the server.
data class YTTrack(
    val videoId: String,
    val title: String,
    val artist: String,
    val duration: Int,
    val thumb: String,
)

// One purchase in the server's queue: queued, buying, done or failed.
data class Buy(
    val id: String,
    val artist: String,
    val title: String,
    val status: String,
    val error: String,
    val result: BuyResult?,
) {
    val pending get() = status == "queued" || status == "buying"
}

// What the store provider reports about a purchase that landed.
data class BuyResult(
    val artist: String,
    val album: String,
    val title: String,
    val file: String,
    val alreadyOwned: Boolean,
)

// A web page playing somewhere, which the phone can drive: what it
// calls itself and what it is on, if anything.
data class RemotePlayer(val id: String, val name: String, val state: RemoteState?)

// A web player's state as it last put it. Times are in seconds, as the
// page keeps them; repeat is off, all or one.
data class RemoteState(
    val key: String,
    val title: String,
    val subtitle: String,
    val art: String,
    val playing: Boolean,
    val position: Double,
    val duration: Double,
    val shuffle: Boolean,
    val repeat: String,
    val hasNext: Boolean,
    val blocked: Boolean,
    val upNext: List<RemoteTrack>,
)

// One track coming up on a web player, and where it is in its queue.
data class RemoteTrack(val index: Int, val title: String, val subtitle: String, val art: String)

// An answer other than 2xx, with what the server said about it.
class Move(val from: String, val to: String, val album: Boolean)

class HttpException(val code: Int, message: String) : IOException(message)

object Api {
    // Set from settings before the first call.
    @Volatile
    var baseUrl = ""

    // A mix or a search runs yt-dlp on the server, which takes a while;
    // nothing here should take longer than this to start answering.
    private const val CONNECT_TIMEOUT = 15_000
    private const val READ_TIMEOUT = 60_000

    private fun enc(s: String) = URLEncoder.encode(s, "UTF-8")

    fun audioUrl(path: String) = "$baseUrl/api/audio?path=${enc(path)}"

    fun streamUrl(videoId: String) = "$baseUrl/api/yt/stream?v=${enc(videoId)}"

    fun coverUrl(artist: String, album: String) = "$baseUrl/api/cover?artist=${enc(artist)}&album=${enc(album)}"

    fun apkUrl() = "$baseUrl/api/apk"

    // open connects with the timeouts every call here wants.
    fun open(url: String): HttpURLConnection =
        (URL(url).openConnection() as HttpURLConnection).apply {
            connectTimeout = CONNECT_TIMEOUT
            readTimeout = READ_TIMEOUT
        }

    // request runs one call and reads its body, raising the server's own
    // words on anything but success.
    private fun request(
        path: String,
        method: String = "GET",
        body: JSONObject? = null,
        headers: (HttpURLConnection) -> Unit = {},
    ): String {
        val conn = open("$baseUrl$path")
        try {
            conn.requestMethod = method
            if (body != null) {
                conn.doOutput = true
                conn.setRequestProperty("Content-Type", "application/json")
                conn.outputStream.use { it.write(body.toString().toByteArray()) }
            }
            val code = conn.responseCode
            if (code !in 200..299) {
                val why = conn.errorStream?.bufferedReader()?.use { it.readText() }?.trim().orEmpty()
                throw HttpException(code, why.ifEmpty { "the server answered $code" })
            }
            headers(conn)
            return conn.inputStream.bufferedReader().use { it.readText() }
        } finally {
            conn.disconnect()
        }
    }

    private inline fun <T> list(a: JSONArray, item: (JSONObject) -> T): List<T> =
        List(a.length()) { item(a.getJSONObject(it)) }

    fun library(): List<Artist> = list(JSONArray(request("/api/library"))) { a ->
        Artist(
            name = a.getString("name"),
            albums = list(a.getJSONArray("albums")) { al ->
                Album(
                    name = al.getString("name"),
                    tracks = list(al.getJSONArray("tracks"), ::parseTrack),
                )
            },
        )
    }

    private fun parseTrack(t: JSONObject) = Track(
        t.getString("name"),
        t.getString("path"),
        t.getLong("size"),
        t.optInt("plays"),
        t.optBoolean("excluded"),
        t.optDouble("duration", 0.0),
    )

    private fun parseOwned(t: JSONObject) = Owned(parseTrack(t), t.getString("artist"), t.getString("album"))

    // health asks a server whether it is one of these, before the app
    // settles on it.
    fun health(base: String) {
        val conn = open("$base/api/health")
        try {
            if (conn.responseCode !in 200..299) throw HttpException(conn.responseCode, "the server answered ${conn.responseCode}")
        } finally {
            conn.disconnect()
        }
    }

    // ----------------------------------------------------------- playlists

    fun playlists(): List<Playlist> = list(JSONArray(request("/api/playlists")), ::parsePlaylist)

    fun playlist(id: String): Playlist = parsePlaylist(JSONObject(request("/api/playlists/${enc(id)}")))

    // savePlaylist creates a playlist, or replaces the one with this id.
    fun savePlaylist(id: String?, name: String, paths: List<String>, smart: Smart?): Playlist {
        val body = JSONObject().put("name", name).put("paths", JSONArray(paths))
        if (smart != null) {
            val rules = JSONArray()
            for (r in smart.rules) rules.put(JSONObject().put("field", r.field).put("op", r.op).put("value", r.value))
            body.put(
                "smart",
                JSONObject().put("match", smart.match).put("rules", rules).put("sort", smart.sort).put("limit", smart.limit),
            )
        }
        val o = if (id == null) request("/api/playlists", "POST", body) else request("/api/playlists/${enc(id)}", "PUT", body)
        return parsePlaylist(JSONObject(o))
    }

    fun addToPlaylist(id: String, paths: List<String>) {
        request("/api/playlists/${enc(id)}/add", "POST", JSONObject().put("paths", JSONArray(paths)))
    }

    fun deletePlaylist(id: String) {
        request("/api/playlists/${enc(id)}", "DELETE")
    }

    private fun parsePlaylist(o: JSONObject): Playlist {
        val paths = o.optJSONArray("paths")
        return Playlist(
            id = o.getString("id"),
            name = o.getString("name"),
            paths = if (paths == null) emptyList() else List(paths.length()) { paths.getString(it) },
            smart = o.optJSONObject("smart")?.let { sm ->
                Smart(
                    sm.optString("match", "all"),
                    sm.optJSONArray("rules")?.let { a ->
                        list(a) { Rule(it.optString("field"), it.optString("op"), it.optString("value")) }
                    }.orEmpty(),
                    sm.optString("sort", "artist"),
                    sm.optInt("limit"),
                )
            },
            count = o.optInt("count"),
            tracks = o.optJSONArray("tracks")?.let { list(it, ::parseOwned) },
        )
    }

    // ----------------------------------------------------- mixes and plays

    // libraryMix is the day's mix of owned tracks, the same all day.
    // supermixReady is whether the server has YouTube Music set up, so
    // the supermix is worth offering.
    fun supermixReady(): Boolean = JSONObject(request("/api/mix")).optBoolean("supermix")

    // A remix other than 0 is another of the day's mixes.
    fun libraryMix(size: Int, remix: Int = 0): List<Owned> =
        list(JSONArray(request("/api/mix/library?size=$size${if (remix != 0) "&remix=$remix" else ""}")), ::parseOwned)

    // madeMixes are the day's mixes made for you.
    fun madeMixes(): List<MadeMix> = list(JSONArray(request("/api/mixes"))) { o ->
        val covers = o.optJSONArray("covers")
        MadeMix(
            id = o.getString("id"),
            title = o.getString("title"),
            tracks = o.optInt("tracks"),
            covers = (0 until (covers?.length() ?: 0)).map { i ->
                val c = covers!!.getJSONArray(i)
                c.getString(0) to c.getString(1)
            },
            blurb = o.optString("blurb"),
        )
    }

    // madeMix is a made mix's tracks for the day.
    fun madeMix(id: String, size: Int): List<Owned> =
        list(JSONObject(request("/api/mixes/${enc(id).replace("+", "%20")}?size=$size")).getJSONArray("tracks"), ::parseOwned)

    // played counts a play of an owned track toward its smart playlists
    // and the library mix.
    fun played(path: String) {
        request("/api/played", "POST", JSONObject().put("path", path))
    }

    // excluded is what is kept out of the mixes: track paths, and yt:<id>
    // for streams.
    // moves is the server's renames after since, and the last one's
    // number: tracks' paths, and albums' "Artist/Album" when album.
    fun moves(since: Int): Pair<Int, List<Move>> {
        val o = JSONObject(request("/api/moves?since=$since"))
        val moves = list(o.getJSONArray("moves")) {
            Move(it.getString("from"), it.getString("to"), it.optBoolean("album"))
        }
        return o.getInt("seq") to moves
    }

    fun excluded(): Set<String> {
        val a = JSONArray(request("/api/excluded"))
        return HashSet<String>().apply { for (i in 0 until a.length()) add(a.getString(i)) }
    }

    fun setExcluded(key: String, excluded: Boolean) {
        request("/api/excluded", "PUT", JSONObject().put("key", key).put("excluded", excluded))
    }

    fun app(): AppUpdate? = runCatching {
        val o = JSONObject(request("/api/app"))
        AppUpdate(o.getString("version"), o.getLong("size"))
    }.getOrNull()

    fun ytSearch(query: String): List<YTTrack> = yt("/api/yt/search?q=${enc(query)}")

    // ytSupermix is the supermix and where it came from: "account", the
    // YouTube Music account's own, or "radio", seeded from the library.
    fun ytSupermix(): Pair<List<YTTrack>, String> {
        var source = ""
        val body = request("/api/yt/supermix") { source = it.getHeaderField("X-Mix-Source").orEmpty() }
        return parseYT(body) to source
    }

    // ytAccountFailed is why the account's own mix last failed, if it did.
    fun ytAccountFailed(): String = JSONObject(request("/api/yt/account")).optString("failed")

    private fun yt(path: String): List<YTTrack> = parseYT(request(path))

    private fun parseYT(body: String): List<YTTrack> = list(JSONArray(body)) { o ->
        YTTrack(
            o.getString("videoId"),
            o.getString("title"),
            o.getString("artist"),
            o.optInt("duration"),
            o.optString("thumb"),
        )
    }

    // ------------------------------------------------------------- remote

    // remoteEvents opens the server's event stream as a watcher; read it
    // with readRemote. The server pings it every 20 seconds, so a read
    // that waits much longer than that is on a dead connection.
    fun remoteEvents(): HttpURLConnection = open("$baseUrl/api/remote/events").apply {
        readTimeout = 45_000
        setRequestProperty("Accept", "text/event-stream")
    }

    // readRemote hands on each list of players the stream sends, until it
    // ends or fails.
    fun readRemote(conn: HttpURLConnection, players: (List<RemotePlayer>) -> Unit) {
        if (conn.responseCode !in 200..299) throw HttpException(conn.responseCode, "the server answered ${conn.responseCode}")
        var event = ""
        val data = StringBuilder()
        conn.inputStream.bufferedReader().useLines { lines ->
            for (line in lines) {
                when {
                    line.isEmpty() -> {
                        if (event == "players") players(list(JSONArray(data.toString()), ::parseRemote))
                        event = ""
                        data.clear()
                    }
                    line.startsWith("event:") -> event = line.removePrefix("event:").trim()
                    line.startsWith("data:") -> data.append(line.removePrefix("data:").trimStart())
                }
            }
        }
    }

    // remote sends a web player a command: do is what (toggle, play,
    // pause, next, prev, seek, jump, shuffle, repeat, owned, streams),
    // and the rest is what it takes.
    fun remote(id: String, command: JSONObject) {
        request("/api/remote/players/${enc(id)}/command", "POST", command)
    }

    private fun parseRemote(o: JSONObject): RemotePlayer {
        val s = o.optJSONObject("state")
        return RemotePlayer(
            o.getString("id"),
            o.optString("name"),
            s?.let {
                RemoteState(
                    it.optString("key"),
                    it.optString("title"),
                    it.optString("sub"),
                    it.optString("art"),
                    it.optBoolean("playing"),
                    it.optDouble("position", 0.0),
                    it.optDouble("duration", 0.0),
                    it.optBoolean("shuffle"),
                    it.optString("repeat", "off"),
                    it.optBoolean("hasNext"),
                    it.optBoolean("blocked"),
                    it.optJSONArray("upNext")?.let { a ->
                        list(a) { t -> RemoteTrack(t.optInt("i"), t.optString("name"), t.optString("sub"), t.optString("art")) }
                    }.orEmpty(),
                )
            },
        )
    }

    // buy queues a streamed track with the server's store provider; the
    // buy runs there, and buys() says where it stands.
    fun buy(track: YTTrack): Buy =
        parseBuy(JSONObject(request("/api/buy", "POST", JSONObject().put("artist", track.artist).put("title", track.title))))

    // buys is the server's purchase queue, newest first.
    fun buys(): List<Buy> = list(JSONArray(request("/api/buys")), ::parseBuy)

    private fun parseBuy(o: JSONObject): Buy {
        val r = o.optJSONObject("result")
        return Buy(
            o.getString("id"),
            o.getString("artist"),
            o.getString("title"),
            o.getString("status"),
            o.optString("error"),
            r?.let {
                BuyResult(
                    it.optString("artist"),
                    it.optString("album"),
                    it.optString("title"),
                    it.optString("file"),
                    it.optBoolean("alreadyOwned"),
                )
            },
        )
    }
}
