package com.jmelahman.music

import android.content.Context
import java.io.File

// Where downloads live: the app's own storage, mirroring the server's
// Artist/Album layout, so a track's identity is the same on both sides.
object Downloads {
    // The tag every download carries, so the queue can be counted, and
    // the one each also carries for its album, so an album's page can
    // follow only its own.
    const val TAG = "music-download"

    fun albumTag(artist: Artist, album: Album) = albumTag(artist.name, album.name)

    fun albumTag(artist: String, album: String) = "album:$artist/$album"

    private fun root(context: Context) = File(context.filesDir, "music")

    // fileFor is where a track is kept, or null for a path that would
    // land outside the library, which the server should never send.
    fun fileFor(context: Context, path: String): File? {
        val root = root(context).canonicalFile
        val f = File(root, path).canonicalFile
        return if (f.path.startsWith(root.path + File.separator)) f else null
    }

    fun saved(context: Context, track: Track): Boolean =
        (fileFor(context, track.path)?.length() ?: 0) > 0

    // onPhone is every track on the phone, by path: one walk of the
    // folder, for a background thread.
    fun onPhone(context: Context): Set<String> {
        val root = root(context)
        return root.walkTopDown()
            .filter { it.isFile && it.length() > 0 && !it.name.endsWith(".part") }
            .mapTo(HashSet()) { it.relativeTo(root).path }
    }

    // savedPaths is which of an album's tracks are on the phone; a disk
    // read per track, so it is for a background thread.
    fun savedPaths(context: Context, album: Album): Set<String> =
        album.tracks.filter { saved(context, it) }.mapTo(HashSet()) { it.path }

    // move follows a track the server renamed, so it needn't be fetched
    // again. A copy already at the new path, as after a merge, stands.
    fun move(context: Context, from: String, to: String) {
        val src = fileFor(context, from) ?: return
        val dst = fileFor(context, to) ?: return
        if (!src.isFile) return
        if (dst.exists()) {
            src.delete()
        } else {
            dst.parentFile?.mkdirs()
            src.renameTo(dst)
        }
        val root = root(context).canonicalFile
        var dir = src.parentFile
        while (dir != null && dir != root && dir.list().isNullOrEmpty()) {
            dir.delete()
            dir = dir.parentFile
        }
    }

    // prune deletes every saved track not in keep, and the folders it
    // leaves empty. A download still being written is left to finish.
    fun prune(context: Context, keep: Set<String>): Int {
        val root = root(context).canonicalFile
        if (!root.isDirectory) return 0
        var n = 0
        root.walkBottomUp().forEach { f ->
            when {
                f == root -> {}
                f.isDirectory -> if (f.list().isNullOrEmpty()) f.delete()
                f.name.endsWith(".part") -> {}
                f.relativeTo(root).invariantSeparatorsPath !in keep -> if (f.delete()) n++
            }
        }
        return n
    }
}
