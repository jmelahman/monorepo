package com.jmelahman.music

import android.content.Context

// The app's own settings, in the platform's own store. allowMetered is
// the whole reason this client is native: streaming on a metered network
// is a choice the user makes once, not something each tap has to
// remember.
class Settings(context: Context) {
    private val prefs = context.getSharedPreferences("music", Context.MODE_PRIVATE)

    var baseUrl: String
        get() = prefs.getString("base_url", Api.baseUrl) ?: Api.baseUrl
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

    companion object {
        fun metered(context: Context): Boolean {
            val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE)
                as? android.net.ConnectivityManager ?: return false
            return cm.isActiveNetworkMetered
        }
    }
}
