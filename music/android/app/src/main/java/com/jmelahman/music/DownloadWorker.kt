package com.jmelahman.music

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL

// One track, fetched into the app's library. The network it may run on
// is decided by the constraint the enqueue attached, which the OS
// enforces: unmetered unless the user said otherwise. A partial file is
// cleaned up so nothing half-written is ever mistaken for saved.
class DownloadWorker(context: Context, params: WorkerParameters) :
    CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val url = inputData.getString("url") ?: return Result.failure()
        val rel = inputData.getString("dest") ?: return Result.failure()
        val dest = Downloads.fileFor(applicationContext, Track("", rel, 0))
        val part = File(dest.path + ".part")
        return withContext(Dispatchers.IO) {
            try {
                val conn = URL(url).openConnection() as HttpURLConnection
                conn.connectTimeout = 15_000
                conn.readTimeout = 60_000
                if (conn.responseCode !in 200..299) {
                    conn.disconnect()
                    return@withContext retry()
                }
                dest.parentFile?.mkdirs()
                part.outputStream().use { conn.inputStream.copyTo(it) }
                conn.disconnect()
                if (part.renameTo(dest)) Result.success() else {
                    part.delete()
                    retry()
                }
            } catch (e: Exception) {
                part.delete()
                retry()
            }
        }
    }

    private fun retry() =
        if (runAttemptCount < 3) Result.retry() else Result.failure()
}
