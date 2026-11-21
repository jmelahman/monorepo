package com.jmelahman.music

import android.content.Context
import android.net.ConnectivityManager
import org.json.JSONArray

// The app's own settings, in the platform's own store. allowMetered is
// the whole reason this client is native: streaming on a metered network
// is a choice the user makes once, not something each tap has to
// remember.
class Settings(context: Context) {
    private val prefs = context.getSharedPreferences("music", Context.MODE_PRIVATE)

    // The server, empty until the first launch asks for it. A build may
    // carry a default (the musicServer gradle property, kept out of the
    // repository) so a phone of one's own needn't be told.
    var baseUrl: String
        get() = prefs.getString("base_url", null) ?: BuildConfig.DEFAULT_SERVER
        set(value) {
            prefs.edit().putString("base_url", value).apply()
            Api.baseUrl = value
        }

    var allowMetered: Boolean
        get() = prefs.getBoolean("allow_metered", false)
        set(value) = prefs.edit().putBoolean("allow_metered", value).apply()

    var allowDownloadsMetered: Boolean
        get() = prefs.getBoolean("downloads_metered", false)
        set(value) = prefs.edit().putBoolean("downloads_metered", value).apply()

    // What the phone keeps in step with the server on Wi-Fi: playlist:<id>,
    // album:<artist>/<album>, and mix:library for the day's library mix.
    var kept: Set<String>
        get() = prefs.getStringSet("kept", null) ?: setOf(LIBRARY_MIX)
        set(value) = prefs.edit().putStringSet("kept", HashSet(value)).apply()

    // How many tracks the library mix and the supermix hold, from
    // MIX_SIZES.
    var mixSize: Int
        get() = prefs.getInt("mix_size", MIX_SIZES.first())
        set(value) = prefs.edit().putInt("mix_size", value).apply()

    var supermixSize: Int
        get() = prefs.getInt("supermix_size", MIX_SIZES.first())
        set(value) = prefs.edit().putInt("supermix_size", value).apply()

    // Whether the albums saved before there was a kept set have joined it.
    var keptMigrated: Boolean
        get() = prefs.getBoolean("kept_migrated", false)
        set(value) = prefs.edit().putBoolean("kept_migrated", value).apply()

    // The albums last played, newest first, as (artist, album).
    var recent: List<Pair<String, String>>
        get() = runCatching {
            val a = JSONArray(prefs.getString("recent", "[]"))
            List(a.length()) { a.getJSONArray(it).let { p -> p.getString(0) to p.getString(1) } }
        }.getOrDefault(emptyList())
        set(value) {
            val a = JSONArray()
            for ((artist, album) in value) a.put(JSONArray().put(artist).put(album))
            prefs.edit().putString("recent", a.toString()).apply()
        }

    companion object {
        const val LIBRARY_MIX = "mix:library"

        // The lengths the server makes a mix in.
        val MIX_SIZES = listOf(50, 100, 200, 500)

        fun playlistKey(id: String) = "playlist:$id"

        fun albumKey(artist: String, album: String) = "album:$artist/$album"

        fun metered(context: Context): Boolean {
            val cm = context.getSystemService(ConnectivityManager::class.java) ?: return false
            return cm.isActiveNetworkMetered
        }
    }
}
