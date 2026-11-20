package com.jmelahman.music

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException

// One track, fetched into the app's library. The network it may run on
// is decided by the constraint the enqueue attached, which the OS
// enforces: unmetered unless the user said otherwise. A partial file is
// cleaned up so nothing half-written is ever mistaken for saved.
class DownloadWorker(context: Context, params: WorkerParameters) :
    CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val url = inputData.getString("url") ?: return Result.failure()
        val rel = inputData.getString("dest") ?: return Result.failure()
        val dest = Downloads.fileFor(applicationContext, rel) ?: return Result.failure()
        val part = File(dest.path + ".part")
        return withContext(Dispatchers.IO) {
            val conn = Api.open(url)
            try {
                val code = conn.responseCode
                when {
                    code in 200..299 -> {}
                    // Gone from the server, or refused: trying again won't help.
                    code in 400..499 -> return@withContext Result.failure()
                    else -> return@withContext retry()
                }
                dest.parentFile?.mkdirs()
                part.outputStream().use { conn.inputStream.copyTo(it) }
                if (part.renameTo(dest)) Result.success() else retry()
            } catch (e: IOException) {
                retry()
            } finally {
                conn.disconnect()
                part.delete()
            }
        }
    }

    private fun retry() =
        if (runAttemptCount < 3) Result.retry() else Result.failure()
}
