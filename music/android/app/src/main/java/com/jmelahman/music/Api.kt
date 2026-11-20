package com.jmelahman.music

import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

// The library as the server lists it, one pass of org.json over the
// api/ the webapp already speaks. HttpURLConnection everywhere; the app
// keeps to the platform's own HTTP and JSON.
data class Track(val name: String, val path: String, val size: Long)
data class Album(val name: String, val tracks: List<Track>)
data class Artist(val name: String, val albums: List<Album>)

// The Android client's build that the server offers, when it is newer
// than the one asking.
data class AppUpdate(val version: String, val size: Long)

// A playable YouTube track, streamed through the server.
data class YTTrack(
    val videoId: String,
    val title: String,
    val artist: String,
    val duration: Int,
)

object Api {
    // Set from settings before the first call.
    var baseUrl = "http://localhost:8095"

    fun library(): List<Artist> {
        val body = URL("$baseUrl/api/library").openStream().bufferedReader().readText()
        val artists = JSONArray(body)
        return List(artists.length()) { i ->
            val a = artists.getJSONObject(i)
            val albums = a.getJSONArray("albums")
            Artist(
                name = a.getString("name"),
                albums = List(albums.length()) { j ->
                    val al = albums.getJSONObject(j)
                    val tracks = al.getJSONArray("tracks")
                    Album(
                        name = al.getString("name"),
                        tracks = List(tracks.length()) { k -> track(tracks.getJSONObject(k)) },
                    )
                },
            )
        }
    }

    private fun track(o: JSONObject) = Track(
        name = o.getString("name"),
        path = o.getString("path"),
        size = o.getLong("size"),
    )

    fun audioUrl(path: String) = "$baseUrl/api/audio?path=" + URLEncoder.encode(path, "UTF-8")

    fun streamUrl(videoId: String) = "$baseUrl/api/yt/stream?v=" + URLEncoder.encode(videoId, "UTF-8")

    // fetch runs a GET and parses its JSON with one read.
    private inline fun <T> fetch(path: String, parse: (JSONObject) -> T): T =
        parse(JSONObject(URL("$baseUrl$path").openStream().bufferedReader().readText()))

    fun app(): AppUpdate? = runCatching {
        fetch("/api/app") { o -> AppUpdate(o.getString("version"), o.getLong("size")) }
    }.getOrNull()

    fun ytSearch(query: String): List<YTTrack> {
        val body = URL("$baseUrl/api/yt/search?q=" + URLEncoder.encode(query, "UTF-8"))
            .openStream().bufferedReader().readText()
        val a = JSONArray(body)
        return List(a.length()) { i ->
            val o = a.getJSONObject(i)
            YTTrack(o.getString("videoId"), o.getString("title"), o.getString("artist"), o.getInt("duration"))
        }
    }

    fun ytSupermix(): List<YTTrack> = ytFrom("/api/yt/supermix")

    private fun ytFrom(path: String): List<YTTrack> {
        val body = URL("$baseUrl$path").openStream().bufferedReader().readText()
        val a = JSONArray(body)
        return List(a.length()) { i ->
            val o = a.getJSONObject(i)
            YTTrack(o.getString("videoId"), o.getString("title"), o.getString("artist"), o.getInt("duration"))
        }
    }

    // buy queues a streamed track with the server's store provider; the
    // buy runs there, and buys() says where it stands.
    fun buy(track: YTTrack): Buy {
        val payload = JSONObject().put("artist", track.artist).put("title", track.title).toString()
        val conn = URL("$baseUrl/api/buy").openConnection() as HttpURLConnection
        try {
            conn.requestMethod = "POST"
            conn.doOutput = true
            conn.setRequestProperty("Content-Type", "application/json")
            conn.outputStream.use { it.write(payload.toByteArray()) }
            if (conn.responseCode !in 200..299) {
                val why = conn.errorStream?.bufferedReader()?.readText()?.trim().orEmpty()
                throw RuntimeException(why.ifEmpty { "the server answered ${conn.responseCode}" })
            }
            return parseBuy(JSONObject(conn.inputStream.bufferedReader().readText()))
        } finally {
            conn.disconnect()
        }
    }

    // buys is the server's purchase queue, newest first.
    fun buys(): List<Buy> {
        val a = JSONArray(URL("$baseUrl/api/buys").openStream().bufferedReader().readText())
        return List(a.length()) { i -> parseBuy(a.getJSONObject(i)) }
    }

    private fun parseBuy(o: JSONObject): Buy {
        val r = o.optJSONObject("result")
        return Buy(
            o.getString("id"), o.getString("artist"), o.getString("title"),
            o.getString("status"), o.optString("error"),
            r?.let {
                BuyResult(
                    it.optString("artist"), it.optString("album"), it.optString("title"),
                    it.optString("file"), it.optBoolean("alreadyOwned"),
                )
            },
        )
    }
}

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
