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
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// ---------------------------------------------------------------- setup

// SetupScreen asks for the server on first launch: the app carries no
// address of its own, so a build can be shared without the network it
// was made for.
@Composable
fun SetupScreen(model: Model, snackbar: SnackbarHostState) {
    var url by remember { mutableStateOf("") }
    Box(
        Modifier
            .fillMaxSize()
            .background(Palette.Graphite)
            .background(Palette.Metal)
            .systemBarsPadding()
            .padding(24.dp),
    ) {
        Column(Modifier.align(Alignment.Center), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Kicker("Welcome")
            Text("Where is your music?", fontSize = 26.sp, fontWeight = FontWeight.SemiBold)
            Text(
                "The address of your music server, as the phone reaches it: on the LAN or over your VPN.",
                color = Palette.Dim,
            )
            OutlinedTextField(
                value = url,
                onValueChange = { url = it },
                label = { Text("Server") },
                placeholder = { Text("http://host:8095") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                keyboardActions = KeyboardActions(onGo = { model.connect(url) }),
                modifier = Modifier.fillMaxWidth(),
            )
            ChromeButton(
                if (model.connecting) "Connecting…" else "Connect",
                null,
                { model.connect(url) },
                enabled = !model.connecting && url.isNotBlank(),
            )
        }
        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

// ------------------------------------------------------------ the rules

private val TEXT_OPS = linkedMapOf(
    "contains" to "contains",
    "is" to "is",
    "not" to "does not contain",
    "starts" to "starts with",
)

private class Field(val label: String, val ops: Map<String, String>, val number: Boolean = false)

private val FIELDS = linkedMapOf(
    "artist" to Field("Artist", TEXT_OPS),
    "album" to Field("Album", TEXT_OPS),
    "title" to Field("Title", TEXT_OPS),
    "plays" to Field("Plays", linkedMapOf(">" to "more than", "<" to "fewer than", "=" to "exactly"), true),
    "added" to Field("Added", linkedMapOf("within" to "in the last … days", "before" to "more than … days ago"), true),
    "played" to Field(
        "Last played",
        linkedMapOf("within" to "in the last … days", "before" to "not in the last … days"),
        true,
    ),
)

private val SORTS = linkedMapOf(
    "artist" to "Artist",
    "added" to "Recently added",
    "plays" to "Most played",
    "played" to "Recently played",
    "random" to "Shuffled daily",
)

// describe says a smart playlist's rules in a line, as the web does.
fun describe(sm: Smart): String {
    val parts = sm.rules.mapNotNull { r ->
        val f = FIELDS[r.field] ?: return@mapNotNull null
        val op = f.ops[r.op] ?: r.op
        "${f.label.lowercase()} ${if ("…" in op) op.replace("…", r.value) else "$op ${r.value}"}"
    }
    val joined = if (parts.isEmpty()) "everything" else parts.joinToString(if (sm.match == "any") " or " else " and ")
    return joined + if (sm.limit > 0) ", up to ${sm.limit}" else ""
}

// ----------------------------------------------------------- playlists

@Composable
fun PlaylistTile(model: Model, p: Playlist) {
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .clickable { model.go(Screen.PlaylistPage(p.id)) }
            .padding(4.dp),
    ) {
        PlaylistArt(
            p,
            Modifier
                .fillMaxWidth()
                .aspectRatio(1f),
        )
        Spacer(Modifier.height(8.dp))
        Text(p.name, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(
            (if (p.smart != null) "Smart · " else "") + plural(p.count, "track"),
            color = Palette.Dim,
            fontSize = 13.sp,
            maxLines = 1,
        )
    }
}

// PlaylistArt is tinted metal with what kind of list it is.
@Composable
fun PlaylistArt(p: Playlist, modifier: Modifier) {
    Box(
        modifier
            .clip(RoundedCornerShape(6.dp))
            .background(Palette.tinted(p.name))
            .background(Palette.Sheen),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            if (p.smart != null) Glyph.Smart else Glyph.List,
            null,
            tint = Palette.Chalk.copy(alpha = 0.75f),
            modifier = Modifier.fillMaxSize(0.36f),
        )
    }
}

@Composable
fun PlaylistScreen(model: Model, id: String, padding: PaddingValues) {
    LaunchedEffect(id) { model.loadPlaylist(id) }
    val p = model.opened[id]
    var editing by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf(false) }
    if (p == null) {
        Box(Modifier.padding(padding)) { Empty("Reading the playlist…") }
        return
    }
    val tracks = p.tracks.orEmpty()
    val key = Settings.playlistKey(p.id)
    LazyColumn(contentPadding = padding) {
        item {
            Header(
                if (p.smart != null) "Smart playlist" else "Playlist",
                p.name,
                listOfNotNull(plural(tracks.size, "track"), p.smart?.let(::describe)).joinToString(" · "),
                art = { PlaylistArt(p, Modifier.size(200.dp)) },
            ) {
                ChromeButton("Play", Glyph.Play, { model.play(tracks, 0) }, enabled = tracks.isNotEmpty())
                PillButton("Shuffle", Glyph.Shuffle, { model.play(tracks, 0, shuffle = true) }, enabled = tracks.isNotEmpty())
                KeepButton(model, key, p.name)
                Box {
                    var open by remember { mutableStateOf(false) }
                    IconButton(onClick = { open = true }) { Icon(Glyph.More, "More") }
                    DropdownMenu(open, { open = false }, containerColor = Palette.CardHi) {
                        DropdownMenuItem(text = { Text("Edit") }, onClick = {
                            open = false
                            editing = true
                        })
                        DropdownMenuItem(text = { Text("Delete") }, onClick = {
                            open = false
                            deleting = true
                        })
                    }
                }
            }
        }
        if (tracks.isEmpty()) {
            item {
                Empty(
                    if (p.smart != null) "Nothing in the library meets these rules yet."
                    else "Add tracks from any album with ⋮.",
                )
            }
        }
        ownedRows(model, tracks, p)
        item { Spacer(Modifier.height(24.dp)) }
    }
    if (editing) PlaylistEditor(model, p, p.smart != null) { editing = false }
    if (deleting) {
        AlertDialog(
            onDismissRequest = { deleting = false },
            containerColor = Palette.CardHi,
            title = { Text("Delete ${p.name}?") },
            text = { Text("The tracks stay in the library.") },
            confirmButton = {
                TextButton(onClick = {
                    deleting = false
                    model.deletePlaylist(p)
                }) { Text("Delete", color = Palette.Accent) }
            },
            dismissButton = { TextButton(onClick = { deleting = false }) { Text("Cancel") } },
        )
    }
}

@Composable
fun LibraryMixScreen(model: Model, padding: PaddingValues) {
    val mix = model.libraryMix
    LazyColumn(contentPadding = padding) {
        item {
            Header(
                "Made from your library",
                "Library mix",
                "${plural(mix?.size ?: 0, "track")} you own, new each day",
                art = { Disc(160) },
            ) {
                ChromeButton("Play", Glyph.Play, { model.play(mix.orEmpty(), 0) }, enabled = !mix.isNullOrEmpty())
                PillButton("Shuffle", Glyph.Shuffle, { model.play(mix.orEmpty(), 0, shuffle = true) }, enabled = !mix.isNullOrEmpty())
                KeepButton(model, Settings.LIBRARY_MIX, "the library mix")
            }
        }
        if (mix == null) item { Empty("Mixing…") }
        ownedRows(model, mix.orEmpty(), null)
        item { Spacer(Modifier.height(24.dp)) }
    }
}

// KeepButton keeps something on the phone, in step with the server.
@Composable
fun KeepButton(model: Model, key: String, name: String) {
    val on = key in model.kept
    PillButton(if (on) "Kept" else "Keep", if (on) Glyph.Saved else Glyph.Save, { model.keep(key, !on, name) })
}

// ownedRows lists owned tracks with their art, each with its menu.
// Only the first limit are shown; a tap still plays them all.
fun LazyListScope.ownedRows(model: Model, tracks: List<Owned>, playlist: Playlist?, limit: Int = Int.MAX_VALUE) {
    itemsIndexed(tracks.take(limit), key = { i, o -> "${o.track.path}:$i" }) { i, o ->
        val on = model.playing?.id == o.track.path
        val out = o.track.path in model.excluded
        Row(
            Modifier
                .fillMaxWidth()
                .clickable { model.play(tracks, i) }
                .padding(start = 16.dp, top = 8.dp, bottom = 8.dp, end = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Cover(o.album, Api.coverUrl(o.artist, o.album), Modifier.size(48.dp), RoundedCornerShape(4.dp))
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    o.track.title,
                    color = when {
                        on -> Palette.Accent
                        out -> Palette.Dim
                        else -> Palette.Chalk
                    },
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    "${o.artist} · ${o.album}",
                    color = Palette.Dim,
                    fontSize = 13.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            if (out) NoMix()
            TrackMenu(model, o, playlist)
        }
    }
}

// NoMix marks a track kept out of the mixes.
@Composable
fun NoMix() {
    Text(
        "NO MIX",
        color = Palette.Dim,
        fontSize = 10.sp,
        fontWeight = FontWeight.SemiBold,
        letterSpacing = 0.6.sp,
        modifier = Modifier
            .border(1.dp, Palette.Edge, CircleShape)
            .padding(horizontal = 7.dp, vertical = 3.dp),
    )
}

// TrackMenu is an owned track's ⋮: into a playlist, out of this one,
// out of (or back into) the mixes, and to its album.
@Composable
fun TrackMenu(model: Model, o: Owned, playlist: Playlist? = null, toAlbum: Boolean = true) {
    var open by remember { mutableStateOf(false) }
    var naming by remember { mutableStateOf(false) }
    val path = o.track.path
    Box {
        IconButton(onClick = {
            open = true
            if (model.playlists == null) model.loadPlaylists()
        }) { Icon(Glyph.More, "More", tint = Palette.Dim, modifier = Modifier.size(20.dp)) }
        DropdownMenu(open, { open = false }, containerColor = Palette.CardHi) {
            Text(
                "ADD TO PLAYLIST",
                color = Palette.Dim,
                fontSize = 11.sp,
                letterSpacing = 1.2.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            )
            for (p in model.playlists.orEmpty().filter { it.smart == null && it.id != playlist?.id }) {
                MenuItem(p.name, Glyph.List) {
                    open = false
                    model.addToPlaylist(p, listOf(path))
                }
            }
            MenuItem("New playlist…", Glyph.Plus) {
                open = false
                naming = true
            }
            HorizontalDivider(color = Palette.Edge)
            if (playlist != null && playlist.smart == null) {
                MenuItem("Remove from ${playlist.name}", null) {
                    open = false
                    model.removeFromPlaylist(playlist, path)
                }
            }
            val out = path in model.excluded
            MenuItem(if (out) "Allow in mixes" else "Keep out of mixes", null) {
                open = false
                model.exclude(path, !out)
            }
            if (toAlbum) {
                MenuItem("Go to album", null) {
                    open = false
                    model.go(Screen.AlbumPage(o.artist, o.album))
                }
            }
        }
    }
    if (naming) {
        NameDialog("New playlist", "", { naming = false }) { model.createPlaylist(it, listOf(path), null) }
    }
}

// StreamMenu is a stream's ⋮: only whether it may come up in the mixes.
@Composable
fun StreamMenu(model: Model, t: YTTrack) {
    var open by remember { mutableStateOf(false) }
    val key = "yt:${t.videoId}"
    val out = key in model.excluded
    Box {
        IconButton(onClick = { open = true }) {
            Icon(Glyph.More, "More", tint = Palette.Dim, modifier = Modifier.size(20.dp))
        }
        DropdownMenu(open, { open = false }, containerColor = Palette.CardHi) {
            MenuItem(if (out) "Allow in mixes" else "Keep out of mixes", null) {
                open = false
                model.exclude(key, !out)
            }
        }
    }
}

@Composable
private fun MenuItem(text: String, icon: ImageVector?, onClick: () -> Unit) {
    DropdownMenuItem(
        text = { Text(text, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        leadingIcon = icon?.let { { Icon(it, null, Modifier.size(18.dp)) } },
        onClick = onClick,
    )
}

@Composable
fun NameDialog(title: String, initial: String, dismiss: () -> Unit, done: (String) -> Unit) {
    var name by remember { mutableStateOf(initial) }
    val save = {
        if (name.isNotBlank()) {
            done(name.trim())
            dismiss()
        }
    }
    AlertDialog(
        onDismissRequest = dismiss,
        containerColor = Palette.CardHi,
        title = { Text(title) },
        text = {
            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text("Name") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { save() }),
            )
        },
        confirmButton = { TextButton(onClick = save, enabled = name.isNotBlank()) { Text("Save") } },
        dismissButton = { TextButton(onClick = dismiss) { Text("Cancel") } },
    )
}

// PlaylistEditor names a playlist and, for a smart one, sets its rules,
// order and size. p is null for a new one.
@Composable
fun PlaylistEditor(model: Model, p: Playlist?, smart: Boolean, dismiss: () -> Unit) {
    if (!smart) {
        NameDialog(if (p == null) "New playlist" else "Rename", p?.name.orEmpty(), dismiss) { name ->
            if (p == null) model.createPlaylist(name, emptyList(), null) else model.savePlaylist(p.id, name, p.paths, null)
        }
        return
    }
    val sm = p?.smart ?: Smart(rules = listOf(Rule("artist", "contains", "")))
    var name by remember { mutableStateOf(p?.name.orEmpty()) }
    var match by remember { mutableStateOf(sm.match) }
    val rules = remember { mutableStateListOf(*sm.rules.toTypedArray()) }
    var sort by remember { mutableStateOf(sm.sort) }
    var limit by remember { mutableStateOf(if (sm.limit > 0) sm.limit.toString() else "") }

    AlertDialog(
        onDismissRequest = dismiss,
        containerColor = Palette.CardHi,
        title = { Text(if (p == null) "New smart playlist" else "Edit ${p.name}") },
        text = {
            Column(
                Modifier.verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                OutlinedTextField(name, { name = it }, label = { Text("Name") }, singleLine = true)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Tracks that meet ")
                    Picker(mapOf("all" to "all", "any" to "any"), match) { match = it }
                    Text(" of these:")
                }
                rules.forEachIndexed { i, r ->
                    RuleRow(r, { rules[i] = it }, { rules.removeAt(i) })
                }
                TextButton(onClick = { rules.add(Rule("artist", "contains", "")) }) {
                    Icon(Glyph.Plus, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(6.dp))
                    Text("Add a rule")
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Order ")
                    Picker(SORTS, sort) { sort = it }
                }
                OutlinedTextField(
                    limit,
                    { v -> limit = v.filter { it.isDigit() }.take(4) },
                    label = { Text("Up to (blank for all)") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                )
            }
        },
        confirmButton = {
            TextButton(
                enabled = name.isNotBlank(),
                onClick = {
                    val s = Smart(match, rules.toList(), sort, limit.toIntOrNull() ?: 0)
                    if (p == null) {
                        model.createPlaylist(name.trim(), emptyList(), s)
                    } else {
                        model.savePlaylist(p.id, name.trim(), emptyList(), s)
                    }
                    dismiss()
                },
            ) { Text("Save") }
        },
        dismissButton = { TextButton(onClick = dismiss) { Text("Cancel") } },
    )
}

@Composable
private fun RuleRow(r: Rule, change: (Rule) -> Unit, remove: () -> Unit) {
    val field = FIELDS[r.field] ?: FIELDS.getValue("artist")
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(Palette.Card)
            .padding(start = 8.dp, top = 4.dp, bottom = 8.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Picker(FIELDS.mapValues { it.value.label }, r.field) { f ->
                val ops = FIELDS.getValue(f).ops
                change(Rule(f, if (r.op in ops) r.op else ops.keys.first(), if (FIELDS.getValue(f).number == field.number) r.value else ""))
            }
            Picker(field.ops, r.op) { change(r.copy(op = it)) }
            Spacer(Modifier.weight(1f))
            IconButton(onClick = remove) { Icon(Glyph.Close, "Remove the rule", Modifier.size(18.dp)) }
        }
        OutlinedTextField(
            r.value,
            { v -> change(r.copy(value = if (field.number) v.filter { it.isDigit() } else v)) },
            placeholder = { Text(if (field.number) "Number" else "Text") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = if (field.number) KeyboardType.Number else KeyboardType.Text),
            modifier = Modifier
                .fillMaxWidth()
                .padding(end = 8.dp),
        )
    }
}

// Picker is a choice of a few, as a text button that opens them.
@Composable
fun Picker(options: Map<String, String>, value: String, pick: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        TextButton(onClick = { open = true }) { Text((options[value] ?: value) + " ▾", color = Palette.Chalk) }
        DropdownMenu(open, { open = false }, containerColor = Palette.CardHi) {
            for ((k, label) in options) {
                DropdownMenuItem(text = { Text(label) }, onClick = {
                    open = false
                    pick(k)
                })
            }
        }
    }
}
