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

    fun albumTag(artist: Artist, album: Album) = "album:${artist.name}/${album.name}"

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

    // savedPaths is which of an album's tracks are on the phone; a disk
    // read per track, so it is for a background thread.
    fun savedPaths(context: Context, album: Album): Set<String> =
        album.tracks.filter { saved(context, it) }.mapTo(HashSet()) { it.path }

    fun delete(context: Context, album: Album) {
        for (t in album.tracks) fileFor(context, t.path)?.delete()
    }
}
