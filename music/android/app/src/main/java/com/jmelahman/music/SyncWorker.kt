package com.jmelahman.music

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.IOException
import java.util.concurrent.TimeUnit

// SyncWorker keeps the phone in step with what it was asked to keep: the
// albums, playlists and library mix in Settings.kept. It queues what is
// missing, under the same network rule as any download, and deletes what
// nothing keeps any more. Anything it can't read from the server stops
// it before a single file is deleted.
class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        val settings = Settings(applicationContext)
        if (settings.baseUrl.isEmpty()) return@withContext Result.success()
        Api.baseUrl = settings.baseUrl
        try {
            followMoves(settings)
            val wanted = wanted(settings)
            Sync.download(applicationContext, settings, wanted.values)
            Downloads.prune(applicationContext, wanted.keys)
            Result.success()
        } catch (e: IOException) {
            if (runAttemptCount < 3) Result.retry() else Result.failure()
        } catch (e: org.json.JSONException) {
            Result.failure()
        }
    }

    // followMoves makes the server's renames to the phone's copies and
    // to the albums it keeps, before anything is pruned for missing.
    private fun followMoves(settings: Settings) {
        val (seq, moves) = try {
            Api.moves(settings.movesSeq)
        } catch (e: HttpException) {
            if (e.code == 404) return // a server from before edits
            throw e
        }
        if (moves.isNotEmpty()) {
            var kept = settings.kept
            var recent = settings.recent
            for (m in moves) {
                if (m.album) {
                    val from = "album:${m.from}"
                    if (from in kept) kept = kept - from + "album:${m.to}"
                    val was = m.from.substringBefore('/') to m.from.substringAfter('/')
                    val now = m.to.substringBefore('/') to m.to.substringAfter('/')
                    recent = recent.map { if (it == was) now else it }.distinct()
                } else {
                    Downloads.move(applicationContext, m.from, m.to)
                }
            }
            settings.kept = kept
            settings.recent = recent
        }
        settings.movesSeq = seq
    }

    // wanted is every track something kept holds, by path. A kept album
    // or playlist the server no longer has is let go.
    private fun wanted(settings: Settings): Map<String, Owned> {
        val library = Api.library()
        val ctx = applicationContext
        var kept = settings.kept
        if (!settings.keptMigrated) {
            // Albums saved before there was a kept set are kept now.
            val saved = library.flatMap { a ->
                a.albums.filter { al -> al.tracks.any { Downloads.saved(ctx, it) } }
                    .map { Settings.albumKey(a.name, it.name) }
            }
            kept = kept + saved
            settings.kept = kept
            settings.keptMigrated = true
        }

        val out = LinkedHashMap<String, Owned>()
        val gone = HashSet<String>()
        for (key in kept) {
            when {
                key == Settings.LIBRARY_MIX -> Api.libraryMix(settings.mixSize).forEach { out[it.track.path] = it }
                key.startsWith("playlist:") -> try {
                    Api.playlist(key.removePrefix("playlist:")).tracks.orEmpty().forEach { out[it.track.path] = it }
                } catch (e: HttpException) {
                    if (e.code != 404) throw e
                    gone += key
                }
                key.startsWith("album:") -> {
                    val found = library.firstNotNullOfOrNull { a ->
                        a.albums.find { Settings.albumKey(a.name, it.name) == key }?.let { a to it }
                    }
                    if (found == null) {
                        gone += key
                    } else {
                        for (t in found.second.tracks) out[t.path] = Owned(t, found.first.name, found.second.name)
                    }
                }
            }
        }
        if (gone.isNotEmpty()) settings.kept = settings.kept - gone
        return out
    }
}

object Sync {
    private const val PERIODIC = "sync"
    private const val NOW = "sync-now"

    // schedule runs the sync a few times a day, whenever there is a
    // network; the downloads it queues wait for the one they may use.
    fun schedule(context: Context) {
        val request = PeriodicWorkRequestBuilder<SyncWorker>(6, TimeUnit.HOURS)
            .setConstraints(connected())
            .build()
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(PERIODIC, ExistingPeriodicWorkPolicy.KEEP, request)
    }

    // now syncs as soon as there is a network, after something kept
    // changed or the app opened.
    fun now(context: Context) {
        val request = OneTimeWorkRequestBuilder<SyncWorker>().setConstraints(connected()).build()
        WorkManager.getInstance(context).enqueueUniqueWork(NOW, ExistingWorkPolicy.REPLACE, request)
    }

    private fun connected() = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

    // download queues one piece of work per missing track, each carrying
    // the constraint the OS then enforces: unmetered networks only, unless
    // the user switched downloads on mobile data on.
    fun download(context: Context, settings: Settings, tracks: Collection<Owned>) {
        val wm = WorkManager.getInstance(context)
        val network = if (settings.allowDownloadsMetered) NetworkType.CONNECTED else NetworkType.UNMETERED
        val missing = tracks.filter { !Downloads.saved(context, it.track) }
        Progress.queued(context, missing.map { it.track.path })
        for (o in missing) {
            val request = OneTimeWorkRequestBuilder<DownloadWorker>()
                .setInputData(workDataOf("url" to Api.audioUrl(o.track.path), "dest" to o.track.path))
                .setConstraints(Constraints.Builder().setRequiredNetworkType(network).build())
                .addTag(Downloads.TAG)
                .addTag(Downloads.albumTag(o.artist, o.album))
                .build()
            wm.enqueueUniqueWork("download:${o.track.path}", ExistingWorkPolicy.KEEP, request)
        }
    }
}
