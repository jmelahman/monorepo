package com.jmelahman.music

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

// Progress is the notification that follows what the phone is saving:
// how many of the tracks it set out to keep have landed, and then that
// they all have. The tracks still to come are remembered by path, so a
// sync that asks again for one already on its way doesn't count it
// twice; the count starts over once nothing is left to come.
object Progress {
    private const val CHANNEL = "downloads"
    private const val ID = 2

    fun queued(context: Context, paths: Collection<String>) = update(context) { s ->
        if (s.pending.isEmpty()) {
            s.done = 0
            s.failed = 0
        }
        s.pending += paths
    }

    fun finished(context: Context, path: String, ok: Boolean) = update(context) { s ->
        if (s.pending.remove(path)) {
            if (ok) s.done++ else s.failed++
        }
    }

    private class State(var done: Int, var failed: Int, val pending: MutableSet<String>)

    @Synchronized
    private fun update(context: Context, change: (State) -> Unit) {
        val prefs = context.getSharedPreferences("progress", Context.MODE_PRIVATE)
        val s = State(
            prefs.getInt("done", 0),
            prefs.getInt("failed", 0),
            prefs.getStringSet("pending", emptySet())!!.toMutableSet(),
        )
        val before = s.pending.size
        change(s)
        prefs.edit()
            .putInt("done", s.done)
            .putInt("failed", s.failed)
            .putStringSet("pending", s.pending)
            .apply()
        if (s.pending.isEmpty() && before == 0) return
        show(context, s)
    }

    private fun show(context: Context, s: State) {
        val nm = NotificationManagerCompat.from(context)
        if (!nm.areNotificationsEnabled()) return
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, "Saving music", NotificationManager.IMPORTANCE_LOW),
        )
        val open = PendingIntent.getActivity(
            context,
            0,
            Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val total = s.done + s.failed + s.pending.size
        val b = NotificationCompat.Builder(context, CHANNEL)
            .setContentIntent(open)
            .setOnlyAlertOnce(true)
            .setSilent(true)
        if (s.pending.isNotEmpty()) {
            b.setSmallIcon(android.R.drawable.stat_sys_download)
                .setContentTitle("Saving to the phone")
                .setContentText("${s.done} of ${plural(total, "track")}")
                .setProgress(total, s.done + s.failed, false)
                .setOngoing(true)
        } else {
            b.setSmallIcon(android.R.drawable.stat_sys_download_done)
                .setContentTitle("Saved ${plural(s.done, "track")}")
                .setContentText(if (s.failed > 0) "${plural(s.failed, "track")} couldn't be saved" else "Ready to play offline")
                .setTimeoutAfter(10_000)
        }
        try {
            nm.notify(ID, b.build())
        } catch (e: SecurityException) {
            // Notifications were turned off between the check and here.
        }
    }
}
