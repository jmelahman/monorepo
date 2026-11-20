package com.jmelahman.music

import android.content.Context
import java.io.File

// Where downloads live: the app's own storage, mirroring the server's
// Artist/Album layout, so a track's identity is the same on both sides.
object Downloads {
    private fun root(context: Context) = File(context.filesDir, "music")

    fun fileFor(context: Context, track: Track): File =
        File(root(context), track.path)

    fun saved(context: Context, track: Track): Boolean =
        fileFor(context, track).length() > 0

    fun savedCount(context: Context, album: Album): Int =
        album.tracks.count { saved(context, it) }
}
