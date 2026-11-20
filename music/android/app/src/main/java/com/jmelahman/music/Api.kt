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
data class Track(val name: String, val path: String, val size: Long) {
    // The name without the leading track number a file name carries.
    val title: String get() = name.replace(TRACK_NUMBER, "").ifEmpty { name }
}

data class Album(val name: String, val tracks: List<Track>) {
    val size: Long get() = tracks.sumOf { it.size }
}

data class Artist(val name: String, val albums: List<Album>)

private val TRACK_NUMBER = Regex("""^\d+[\s._-]*""")

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

// An answer other than 2xx, with what the server said about it.
class HttpException(val code: Int, message: String) : IOException(message)

object Api {
    // Set from settings before the first call.
    @Volatile
    var baseUrl = "http://localhost:8095"

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
    private fun request(path: String, method: String = "GET", body: JSONObject? = null): String {
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
                    tracks = list(al.getJSONArray("tracks")) { t ->
                        Track(t.getString("name"), t.getString("path"), t.getLong("size"))
                    },
                )
            },
        )
    }

    fun app(): AppUpdate? = runCatching {
        val o = JSONObject(request("/api/app"))
        AppUpdate(o.getString("version"), o.getLong("size"))
    }.getOrNull()

    fun ytSearch(query: String): List<YTTrack> = yt("/api/yt/search?q=${enc(query)}")

    fun ytSupermix(): List<YTTrack> = yt("/api/yt/supermix")

    private fun yt(path: String): List<YTTrack> = list(JSONArray(request(path))) { o ->
        YTTrack(
            o.getString("videoId"),
            o.getString("title"),
            o.getString("artist"),
            o.optInt("duration"),
            o.optString("thumb"),
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
