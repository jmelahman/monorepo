package com.jmelahman.music

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.work.WorkInfo
import androidx.work.WorkManager
import coil.compose.AsyncImage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.util.Locale
import kotlin.math.roundToInt

// ---------------------------------------------------------------- parts

// Cover is a picture over a placeholder of tinted metal with the name's
// initial, which stays when there is no picture to load.
@Composable
fun Cover(name: String, url: String?, modifier: Modifier = Modifier, shape: Shape = RoundedCornerShape(6.dp)) {
    Box(
        modifier
            .clip(shape)
            .background(Palette.tinted(name))
            .background(Palette.Sheen),
        contentAlignment = Alignment.Center,
    ) {
        val initial = name.firstOrNull { it.isLetterOrDigit() }?.uppercaseChar()?.toString() ?: "♪"
        Text(initial, color = Color(0xCCD8DBE0), fontWeight = FontWeight.SemiBold, fontSize = 28.sp)
        if (url != null) {
            AsyncImage(
                model = url,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

// ChromeButton is a pill for the main action, with the polished face.
@Composable
fun ChromeButton(text: String, icon: ImageVector?, onClick: () -> Unit, enabled: Boolean = true) {
    Row(
        Modifier
            .height(40.dp)
            .clip(CircleShape)
            .background(Palette.Chrome)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 18.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            Icon(icon, null, tint = Palette.OnChrome, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
        }
        Text(text, color = Palette.OnChrome, fontWeight = FontWeight.SemiBold)
    }
}

// PillButton is the quieter one beside it.
@Composable
fun PillButton(text: String, icon: ImageVector?, onClick: () -> Unit, enabled: Boolean = true) {
    Row(
        Modifier
            .height(40.dp)
            .clip(CircleShape)
            .background(Palette.CardHi)
            .border(1.dp, Palette.Edge, CircleShape)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            Icon(icon, null, tint = Palette.Chalk, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
        }
        Text(text, color = if (enabled) Palette.Chalk else Palette.Dim, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun Chip(text: String, selected: Boolean, onClick: () -> Unit) {
    val shape = CircleShape
    Box(
        Modifier
            .clip(shape)
            .then(
                if (selected) Modifier.background(Palette.Chrome)
                else Modifier.background(Palette.Card).border(1.dp, Palette.Edge, shape),
            )
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 7.dp),
    ) {
        Text(
            text,
            color = if (selected) Palette.OnChrome else Palette.Chalk,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            fontSize = 13.sp,
        )
    }
}

@Composable
fun SectionTitle(text: String, modifier: Modifier = Modifier) {
    Text(
        text,
        modifier = modifier.padding(start = 16.dp, end = 16.dp, top = 24.dp, bottom = 12.dp),
        style = MaterialTheme.typography.titleLarge,
        fontWeight = FontWeight.SemiBold,
    )
}

@Composable
fun Kicker(text: String) {
    Text(
        text.uppercase(Locale.ROOT),
        color = Palette.Dim,
        fontSize = 11.sp,
        letterSpacing = 1.8.sp,
        fontWeight = FontWeight.SemiBold,
    )
}

@Composable
fun Empty(text: String) {
    Text(text, color = Palette.Dim, modifier = Modifier.padding(16.dp))
}

@Composable
private fun Tile(title: String, sub: String, url: String?, width: Dp?, round: Boolean = false, onClick: () -> Unit) {
    Column(
        (if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clip(RoundedCornerShape(8.dp))
            .clickable(onClick = onClick)
            .padding(4.dp),
        horizontalAlignment = if (round) Alignment.CenterHorizontally else Alignment.Start,
    ) {
        Cover(
            title,
            url,
            Modifier
                .fillMaxWidth()
                .aspectRatio(1f),
            if (round) CircleShape else RoundedCornerShape(6.dp),
        )
        Spacer(Modifier.height(8.dp))
        Text(title, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(sub, color = Palette.Dim, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun AlbumTile(model: Model, artist: Artist, album: Album, width: Dp? = null) =
    Tile(album.name, artist.name, Api.coverUrl(artist.name, album.name), width) {
        model.go(Screen.AlbumPage(artist.name, album.name))
    }

private fun artistCover(a: Artist) = a.albums.firstOrNull()?.let { Api.coverUrl(a.name, it.name) }

fun plural(n: Int, one: String) = "$n $one${if (n == 1) "" else "s"}"

private fun mb(bytes: Long) = String.format(Locale.ROOT, "%.1f MB", bytes / 1e6)

fun clock(ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    return if (s >= 3600) {
        String.format(Locale.ROOT, "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60)
    } else {
        String.format(Locale.ROOT, "%d:%02d", s / 60, s % 60)
    }
}

@Composable
fun LibraryPending(model: Model) {
    val error = model.loadError
    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (error != null) {
            Text(error, color = Palette.Accent)
            PillButton("Retry", null, { model.reload() })
        } else {
            Text("Reading the library…", color = Palette.Dim)
        }
    }
}

// ----------------------------------------------------------------- home

@Composable
fun HomeScreen(model: Model, padding: PaddingValues) {
    val library = model.library
    // A sample of the library, drawn once per load so it holds still.
    val sample = remember(library) {
        library.orEmpty().flatMap { a -> a.albums.map { a to it } }.shuffled().take(18)
    }
    val recent = model.recent.mapNotNull { (a, al) -> model.album(a, al) }
    var confirming by remember { mutableStateOf<YTTrack?>(null) }

    LazyColumn(contentPadding = padding) {
        model.update?.let { u ->
            item {
                Row(
                    Modifier
                        .padding(16.dp, 12.dp, 16.dp, 0.dp)
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(12.dp))
                        .background(Palette.Card)
                        .padding(14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text("Version ${u.version} is ready", Modifier.weight(1f))
                    ChromeButton(
                        if (model.updating) "Downloading…" else "Update",
                        Glyph.Update,
                        { model.installUpdate() },
                        enabled = !model.updating,
                    )
                }
            }
        }
        if (model.supermixReady) {
            item { MixHero(model) }
        }
        item { LibraryMixHero(model) }
        model.libraryMix?.takeIf { it.isNotEmpty() }?.let { mix ->
            item {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    SectionTitle("Library mix", Modifier.weight(1f))
                    TextButton(onClick = { model.go(Screen.LibraryMix) }) { Text("See all", color = Palette.Dim) }
                }
            }
            ownedRows(model, mix, null, limit = 5)
        }
        model.mix?.takeIf { model.supermixReady }?.let { mix ->
            item { SectionTitle("Supermix") }
            streamRows(model, mix) { confirming = it }
        }
        if (library == null) {
            item { LibraryPending(model) }
        }
        if (recent.isNotEmpty()) {
            item { SectionTitle("Jump back in") }
            item { AlbumRow(model, recent) }
        }
        if (sample.isNotEmpty()) {
            item {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    SectionTitle("From your library", Modifier.weight(1f))
                    TextButton(onClick = { model.tab(Screen.Library()) }) { Text("See all", color = Palette.Dim) }
                }
            }
            item { AlbumRow(model, sample) }
        }
        item { Spacer(Modifier.height(24.dp)) }
    }
    BuyDialog(model, confirming) { confirming = null }
}

@Composable
private fun AlbumRow(model: Model, albums: List<Pair<Artist, Album>>) {
    LazyRow(contentPadding = PaddingValues(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        items(albums, key = { (a, al) -> "${a.name}/${al.name}" }) { (a, al) -> AlbumTile(model, a, al, 148.dp) }
    }
}

// Disc is the mixes' art: a record, brushed.
@Composable
fun Disc(size: Int) {
    Box(
        Modifier
            .size(size.dp)
            .clip(CircleShape)
            .background(
                Brush.sweepGradient(
                    listOf(
                        Color(0xFF2A2B30), Color(0xFF6B6F78), Color(0xFF2A2B30),
                        Color(0xFF8D929B), Color(0xFF2A2B30),
                    ),
                ),
            ),
        contentAlignment = Alignment.Center,
    ) {
        Box(
            Modifier
                .size((size / 6).dp)
                .clip(CircleShape)
                .background(Palette.Card),
        )
    }
}

// MixArt leads a mix or a playlist with four of the covers it will play,
// or one when it has fewer, as the web's does; blank stands in before it
// has any. Each cover is a name, which draws the tile when the picture
// won't, and its picture.
@Composable
fun MixArt(covers: List<Pair<String, String?>>, size: Dp, blank: @Composable () -> Unit) {
    val picked = covers.filter { !it.second.isNullOrEmpty() }.distinctBy { it.second }.take(4)
    val shown = if (picked.size == 4) picked else picked.take(1)
    if (shown.isEmpty()) {
        blank()
        return
    }
    val shape = RoundedCornerShape(if (size > 120.dp) 8.dp else 6.dp)
    Column(
        Modifier
            .size(size)
            .clip(shape)
            .background(Palette.CardHi),
    ) {
        shown.chunked(2).forEach { row ->
            Row(Modifier.weight(1f)) {
                row.forEach { (name, url) ->
                    Cover(name, url, Modifier.weight(1f).fillMaxHeight(), RectangleShape)
                }
            }
        }
    }
}

// MixCard is one of the two mixes on the home page, tinted by its kind:
// the accent for streams, green for what is owned.
@Composable
private fun MixCard(
    kicker: String,
    title: String,
    tint: Color,
    covers: List<Pair<String, String?>>,
    note: String? = null,
    open: (() -> Unit)? = null,
    actions: @Composable () -> Unit,
) {
    val shape = RoundedCornerShape(16.dp)
    Column(
        Modifier
            .padding(start = 16.dp, end = 16.dp, top = 12.dp)
            .fillMaxWidth()
            .clip(shape)
            // Flat enough for the wash to show through, as on the web.
            .background(Palette.Card.copy(alpha = 0.72f))
            .background(
                Brush.radialGradient(
                    listOf(tint.copy(alpha = 0.22f), Color.Transparent),
                    radius = 700f,
                    center = androidx.compose.ui.geometry.Offset(Float.POSITIVE_INFINITY, 0f),
                ),
            )
            .background(Palette.Sheen)
            .border(1.dp, Palette.Edge, shape)
            .padding(18.dp),
    ) {
        Row(
            Modifier.then(if (open != null) Modifier.clickable(onClick = open) else Modifier),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MixArt(covers, 88.dp) { Disc(88) }
            Column(Modifier.padding(start = 16.dp)) {
                Kicker(kicker)
                Text(title, fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
            }
        }
        if (note != null) Text(note, Modifier.padding(top = 10.dp), color = Palette.Dim, fontSize = 13.sp)
        Row(Modifier.padding(top = 14.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) { actions() }
    }
}

// MixHero is the supermix: YouTube Music's own for the account, or a
// radio seeded from the library when the account doesn't answer. It
// loads by itself; asking again brings the same, so there's no remix.
@Composable
private fun MixHero(model: Model) {
    val mix = model.mix
    MixCard(
        if (model.mixSource == "radio") "Seeded from your library" else "From YouTube Music",
        "Supermix",
        Palette.Accent,
        mix.orEmpty().map { it.title to it.thumb },
        note = when {
            model.mixFailed -> "The mix didn't load."
            model.mixSource == "radio" && model.accountFailed.isNotEmpty() -> model.accountFailed
            else -> null
        },
    ) {
        if (model.mixFailed) {
            ChromeButton("Try again", Glyph.Mix, { model.loadMix() })
        } else {
            ChromeButton(
                if (model.mixing) "Mixing…" else "Play",
                Glyph.Play,
                { if (!mix.isNullOrEmpty()) model.playStreams(mix, 0) },
                enabled = !mix.isNullOrEmpty(),
            )
        }
    }
}

// LibraryMixHero is the day's mix of owned tracks, which the phone keeps
// unless told otherwise.
@Composable
private fun LibraryMixHero(model: Model) {
    val mix = model.libraryMix
    MixCard(
        "Made from your library",
        "Library mix",
        Palette.Owned,
        mix.orEmpty().map { it.album to Api.coverUrl(it.artist, it.album) },
        open = { model.go(Screen.LibraryMix) },
    ) {
        ChromeButton("Play", Glyph.Play, { model.play(mix.orEmpty(), 0) }, enabled = !mix.isNullOrEmpty())
        PillButton("Remix", Glyph.Mix, { model.remixLibrary() }, enabled = mix != null)
        KeepButton(model, Settings.LIBRARY_MIX, "the library mix")
    }
}

// SizePicker chooses how many tracks a mix holds.
@Composable
fun SizePicker(size: Int, pick: (Int) -> Unit) {
    Picker(Settings.MIX_SIZES.associate { "$it" to "$it tracks" }, "$size") { pick(it.toInt()) }
}

// -------------------------------------------------------------- library

@Composable
fun LibraryScreen(model: Model, view: Shelf, padding: PaddingValues) {
    val library = model.library
    LazyVerticalGrid(
        columns = GridCells.Adaptive(150.dp),
        contentPadding = PaddingValues(
            start = 12.dp,
            end = 12.dp,
            top = padding.calculateTopPadding(),
            bottom = padding.calculateBottomPadding() + 24.dp,
        ),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item(span = { GridItemSpan(maxLineSpan) }) {
            Row(
                Modifier.padding(4.dp, 16.dp, 4.dp, 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                for (v in Shelf.entries) Chip(v.name, v == view) { model.tab(Screen.Library(v)) }
            }
        }
        if (view == Shelf.Playlists) {
            playlistShelf(model)
        } else if (library == null) {
            item(span = { GridItemSpan(maxLineSpan) }) { LibraryPending(model) }
        } else if (view == Shelf.Artists) {
            items(library, key = { it.name }) { a ->
                Tile(a.name, plural(a.albums.size, "album"), artistCover(a), null, round = true) {
                    model.go(Screen.ArtistPage(a.name))
                }
            }
        } else {
            val albums = library.flatMap { a -> a.albums.map { a to it } }
            items(albums, key = { (a, al) -> "${a.name}/${al.name}" }) { (a, al) -> AlbumTile(model, a, al) }
        }
        if (view != Shelf.Playlists && library != null) {
            item(span = { GridItemSpan(maxLineSpan) }) {
                Text(
                    libraryStats(library),
                    Modifier.fillMaxWidth().padding(top = 20.dp),
                    color = Palette.Dim,
                    fontSize = 13.sp,
                    textAlign = TextAlign.Center,
                )
            }
        }
    }
}

// libraryStats is the line iTunes kept under its library: how many songs,
// how long they play and how much room they take.
private fun libraryStats(library: List<Artist>): String {
    val tracks = library.flatMap { a -> a.albums.flatMap { it.tracks } }
    val secs = tracks.sumOf { it.duration }
    val bytes = tracks.sumOf { it.size }
    val time = when {
        secs >= 86400 -> String.format(Locale.ROOT, "%.1f days", secs / 86400)
        secs >= 3600 -> String.format(Locale.ROOT, "%.1f hours", secs / 3600)
        else -> plural((secs / 60).roundToInt(), "minute")
    }
    val size = if (bytes >= 1L shl 30) {
        String.format(Locale.ROOT, "%.2f GB", bytes / (1L shl 30).toDouble())
    } else {
        String.format(Locale.ROOT, "%.1f MB", bytes / (1L shl 20).toDouble())
    }
    val songs = "%,d song%s".format(tracks.size, if (tracks.size == 1) "" else "s")
    return "$songs, $time, $size"
}

// playlistShelf is the library's playlists, with the two ways to start
// one.
private fun androidx.compose.foundation.lazy.grid.LazyGridScope.playlistShelf(model: Model) {
    item(span = { GridItemSpan(maxLineSpan) }) {
        var making by remember { mutableStateOf<Boolean?>(null) }
        Row(Modifier.padding(4.dp, 0.dp, 4.dp, 8.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            PillButton("New", Glyph.Plus, { making = false })
            PillButton("Smart", Glyph.Smart, { making = true })
        }
        making?.let { PlaylistEditor(model, null, it) { making = null } }
    }
    val lists = model.playlists
    when {
        lists == null -> item(span = { GridItemSpan(maxLineSpan) }) { Empty("Reading the playlists…") }
        lists.isEmpty() -> item(span = { GridItemSpan(maxLineSpan) }) {
            Empty("No playlists yet. Start one, or let a smart one fill itself from rules.")
        }
        else -> items(lists, key = { it.id }) { p -> PlaylistTile(model, p) }
    }
}

// Header is the top of an artist's or album's page: big art, names, and
// what can be done with it all.
@Composable
fun Header(
    kicker: String,
    title: String,
    meta: String,
    art: @Composable () -> Unit,
    actions: @Composable () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        art()
        Spacer(Modifier.height(20.dp))
        Kicker(kicker)
        Text(
            title,
            fontSize = 26.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 4.dp),
        )
        Text(meta, color = Palette.Dim, modifier = Modifier.padding(top = 4.dp, bottom = 16.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) { actions() }
    }
}

@Composable
fun ArtistScreen(model: Model, name: String, padding: PaddingValues) {
    val a = model.artist(name)
    if (a == null) {
        Box(Modifier.padding(padding)) { if (model.library == null) LibraryPending(model) else Empty("No artist “$name”.") }
        return
    }
    val tracks = a.albums.sumOf { it.tracks.size }
    LazyVerticalGrid(
        columns = GridCells.Adaptive(150.dp),
        contentPadding = PaddingValues(
            start = 12.dp,
            end = 12.dp,
            top = padding.calculateTopPadding(),
            bottom = padding.calculateBottomPadding() + 24.dp,
        ),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item(span = { GridItemSpan(maxLineSpan) }) {
            Header(
                "Artist",
                a.name,
                "${plural(a.albums.size, "album")} · ${plural(tracks, "track")}",
                art = { Cover(a.name, artistCover(a), Modifier.size(180.dp), CircleShape) },
            ) {
                ChromeButton("Play", Glyph.Play, { model.playArtist(a, shuffle = false) })
                PillButton("Shuffle", Glyph.Shuffle, { model.playArtist(a, shuffle = true) })
            }
        }
        items(a.albums, key = { it.name }) { al -> AlbumTile(model, a, al) }
    }
}

@Composable
fun AlbumScreen(model: Model, artistName: String, albumName: String, padding: PaddingValues) {
    val found = model.album(artistName, albumName)
    if (found == null) {
        Box(Modifier.padding(padding)) {
            if (model.library == null) LibraryPending(model) else Empty("No album “$albumName”.")
        }
        return
    }
    val (artist, album) = found
    val context = LocalContext.current

    // The album's downloads, as WorkManager reports them, and which of its
    // tracks are on the phone, read again as each one lands.
    val tag = Downloads.albumTag(artist, album)
    val work by remember(tag) { WorkManager.getInstance(context).getWorkInfosByTagFlow(tag) }
        .collectAsState(initial = emptyList())
    val active = work.count { !it.state.isFinished }
    val landed = work.count { it.state == WorkInfo.State.SUCCEEDED }
    var saved by remember(album) { mutableStateOf<Set<String>>(emptySet()) }
    var removed by remember(album) { mutableStateOf(0) }
    LaunchedEffect(album, landed, removed) {
        saved = withContext(Dispatchers.IO) { Downloads.savedPaths(context, album) }
    }
    val all = saved.size == album.tracks.size

    val current = model.playing?.id
    LazyColumn(contentPadding = padding) {
        item {
            Header(
                if (album.name == "Singles") "Singles" else "Album",
                album.name,
                "${artist.name} · ${plural(album.tracks.size, "track")} · ${mb(album.size)}",
                art = {
                    Cover(album.name, Api.coverUrl(artist.name, album.name), Modifier.size(220.dp), RoundedCornerShape(8.dp))
                },
            ) {
                ChromeButton("Play", Glyph.Play, { model.playAlbum(artist, album) })
                PillButton("Shuffle", Glyph.Shuffle, { model.playAlbum(artist, album, shuffle = true) })
                val kept = Settings.albumKey(artist.name, album.name) in model.kept
                when {
                    kept && active > 0 -> PillButton("${saved.size}/${album.tracks.size}", Glyph.Save, {
                        model.removeAlbum(artist, album)
                        removed++
                    })
                    kept -> PillButton("Kept", Glyph.Saved, {
                        model.removeAlbum(artist, album)
                        removed++
                    })
                    else -> PillButton(if (all) "Keep" else "Save", Glyph.Save, { model.saveAlbum(artist, album) })
                }
            }
        }
        item {
            Text(
                "by ${artist.name}",
                color = Palette.Dim,
                fontSize = 13.sp,
                modifier = Modifier
                    .padding(horizontal = 16.dp)
                    .clickable { model.go(Screen.ArtistPage(artist.name)) },
            )
        }
        itemsIndexed(album.tracks, key = { _, t -> t.path }) { i, t ->
            val on = t.path == current
            Row(
                Modifier
                    .fillMaxWidth()
                    .clickable { model.playAlbum(artist, album, i) }
                    .padding(start = 16.dp, top = 4.dp, bottom = 4.dp, end = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.width(32.dp), contentAlignment = Alignment.CenterStart) {
                    if (on) {
                        Meter(model.isPlaying)
                    } else {
                        Text("${i + 1}", color = Palette.Dim, fontSize = 13.sp)
                    }
                }
                Text(
                    t.title,
                    Modifier.weight(1f),
                    color = if (on) Palette.Accent else Palette.Chalk,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (t.path in model.excluded) NoMix()
                if (t.path in saved) {
                    Icon(
                        Glyph.Saved,
                        "Saved",
                        tint = Palette.Dim,
                        modifier = Modifier
                            .padding(start = 8.dp)
                            .size(16.dp),
                    )
                }
                TrackMenu(model, Owned(t, artist.name, album.name), toAlbum = false)
            }
        }
        item { Spacer(Modifier.height(24.dp)) }
    }
}

// --------------------------------------------------------------- search

@Composable
fun SearchScreen(model: Model, padding: PaddingValues) {
    val focus = remember { FocusRequester() }
    var confirming by remember { mutableStateOf<YTTrack?>(null) }
    LaunchedEffect(Unit) { if (model.results == null) focus.requestFocus() }

    LazyColumn(contentPadding = padding) {
        item {
            TextField(
                value = model.query,
                onValueChange = { model.query = it },
                placeholder = { Text("Search songs to stream") },
                leadingIcon = { Icon(Glyph.Search, null, Modifier.size(20.dp)) },
                trailingIcon = {
                    if (model.query.isNotEmpty()) {
                        Icon(
                            Glyph.Close,
                            "Clear",
                            Modifier
                                .size(20.dp)
                                .clickable { model.query = "" },
                        )
                    }
                },
                singleLine = true,
                shape = CircleShape,
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = Palette.Card,
                    unfocusedContainerColor = Palette.Card,
                    focusedIndicatorColor = Color.Transparent,
                    unfocusedIndicatorColor = Color.Transparent,
                    cursorColor = Palette.Accent,
                ),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                keyboardActions = KeyboardActions(onSearch = { model.search(model.query) }),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp)
                    .focusRequester(focus),
            )
        }
        val results = model.results
        when {
            model.searching -> item { Empty("Searching…") }
            results == null -> item { Empty("Find any song; play it now, buy it if it stays.") }
            results.isEmpty() -> item { Empty("Nothing found.") }
            else -> streamRows(model, results) { confirming = it }
        }
    }
    BuyDialog(model, confirming) { confirming = null }
}

// streamRows lists streams with their stills and a buy button each.
private fun LazyListScope.streamRows(model: Model, tracks: List<YTTrack>, buy: (YTTrack) -> Unit) {
    itemsIndexed(tracks, key = { i, t -> "${t.videoId}:$i" }) { i, t ->
        val on = model.playing?.id == "yt:${t.videoId}"
        Row(
            Modifier
                .fillMaxWidth()
                .clickable { model.playStreams(tracks, i) }
                .padding(start = 16.dp, top = 8.dp, bottom = 8.dp, end = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Cover(t.title, t.thumb.ifEmpty { null }, Modifier.size(48.dp), RoundedCornerShape(4.dp))
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    t.title,
                    color = if (on) Palette.Accent else Palette.Chalk,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    listOfNotNull(t.artist, t.duration.takeIf { it > 0 }?.let { clock(it * 1000L) }).joinToString(" · "),
                    color = Palette.Dim,
                    fontSize = 13.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            BuyButton(model.buyFor(t), model.owns(t)) { buy(t) }
            StreamMenu(model, t)
        }
    }
}

// BuyButton says where a track's newest buy stands, and offers one when
// there is none or the last failed.
@Composable
fun BuyButton(buy: Buy?, inLibrary: Boolean, onClick: () -> Unit) {
    val label = if (inLibrary) "Owned" else when (buy?.status) {
        // A queued buy with an error has failed once and waits to try again.
        "queued" -> if (buy?.error.isNullOrEmpty()) "Queued" else "Retrying"
        "buying" -> "Buying…"
        "done" -> "Owned"
        "failed" -> "Retry"
        else -> "Buy"
    }
    val owned = inLibrary || buy?.status == "done"
    val enabled = !owned && (buy == null || buy.status == "failed")
    Box(
        Modifier
            .padding(start = 8.dp)
            .clip(CircleShape)
            .then(
                if (owned) Modifier
                else Modifier.background(Palette.CardHi).border(1.dp, Palette.Edge, CircleShape),
            )
            .clickable(enabled = enabled, onClick = onClick)
            .widthIn(min = 76.dp)
            .padding(horizontal = 14.dp, vertical = 7.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold,
            color = when {
                owned -> Palette.Owned
                enabled -> Palette.Chalk
                else -> Palette.Dim
            },
        )
    }
}

// BuyDialog asks first: a buy spends the store account's balance.
@Composable
fun BuyDialog(model: Model, track: YTTrack?, dismiss: () -> Unit) {
    track ?: return
    AlertDialog(
        onDismissRequest = dismiss,
        containerColor = Palette.CardHi,
        title = { Text("Buy this track?") },
        text = { Text("${track.artist} — ${track.title}, into the owned library.") },
        confirmButton = {
            TextButton(onClick = {
                model.buyTrack(track)
                dismiss()
            }) { Text("Buy", color = Palette.Accent) }
        },
        dismissButton = { TextButton(onClick = dismiss) { Text("Cancel") } },
    )
}

// ------------------------------------------------------------- settings

@Composable
fun SettingsDialog(model: Model, onDismiss: () -> Unit) {
    var url by remember { mutableStateOf(model.settings.baseUrl) }
    var allowMetered by remember { mutableStateOf(model.settings.allowMetered) }
    var allowDownloadsMetered by remember { mutableStateOf(model.settings.allowDownloadsMetered) }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = Palette.CardHi,
        title = { Text("Settings") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text("Server") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = androidx.compose.ui.text.input.KeyboardType.Uri),
                )
                SwitchRow("Stream on mobile data", allowMetered) { allowMetered = it }
                SwitchRow("Download on mobile data", allowDownloadsMetered) { allowDownloadsMetered = it }
                Text("Version ${BuildConfig.VERSION_NAME}", color = Palette.Dim, fontSize = 12.sp)
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    model.saveSettings(url, allowMetered, allowDownloadsMetered)
                    onDismiss()
                },
            ) { Text("Save") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun SwitchRow(text: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(text, modifier = Modifier.weight(1f))
        androidx.compose.material3.Switch(
            checked = checked,
            onCheckedChange = onChange,
            colors = androidx.compose.material3.SwitchDefaults.colors(
                checkedThumbColor = Palette.OnChrome,
                checkedTrackColor = Palette.Chalk,
            ),
        )
    }
}
