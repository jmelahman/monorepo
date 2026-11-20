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

    var baseUrl: String
        get() = prefs.getString("base_url", null) ?: Api.baseUrl
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
        fun metered(context: Context): Boolean {
            val cm = context.getSystemService(ConnectivityManager::class.java) ?: return false
            return cm.isActiveNetworkMetered
        }
    }
}
