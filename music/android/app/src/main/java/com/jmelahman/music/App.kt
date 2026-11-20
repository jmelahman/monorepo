package com.jmelahman.music

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

@Composable
fun MusicApp(model: Model) {
    MusicTheme {
        val snackbar = remember { SnackbarHostState() }
        LaunchedEffect(Unit) { model.noticeFlow.collect { snackbar.showSnackbar(it) } }
        if (model.server.isEmpty()) {
            SetupScreen(model, snackbar)
            return@MusicTheme
        }
        BackHandler(enabled = model.expanded || model.stack.size > 1) { model.back() }
        var settingsOpen by remember { mutableStateOf(false) }

        Box(
            Modifier
                .fillMaxSize()
                .background(Palette.Graphite),
        ) {
            Scaffold(
                containerColor = Palette.Graphite,
                contentWindowInsets = WindowInsets(0),
                topBar = { TopBar(model) { settingsOpen = true } },
                bottomBar = {
                    Column(
                        Modifier
                            .background(Palette.Metal)
                            .navigationBarsPadding(),
                    ) {
                        if (model.playing != null) MiniPlayer(model)
                        NavBar(model)
                    }
                },
                snackbarHost = { SnackbarHost(snackbar) },
            ) { padding ->
                when (val s = model.screen) {
                    Screen.Home -> HomeScreen(model, padding)
                    is Screen.Library -> LibraryScreen(model, s.view, padding)
                    is Screen.ArtistPage -> ArtistScreen(model, s.artist, padding)
                    is Screen.AlbumPage -> AlbumScreen(model, s.artist, s.album, padding)
                    Screen.Search -> SearchScreen(model, padding)
                    is Screen.PlaylistPage -> PlaylistScreen(model, s.id, padding)
                    Screen.LibraryMix -> LibraryMixScreen(model, padding)
                }
            }
            AnimatedVisibility(
                visible = model.expanded && model.playing != null,
                enter = slideInVertically { it },
                exit = slideOutVertically { it },
            ) {
                PlayerSheet(model)
            }
        }
        if (settingsOpen) SettingsDialog(model) { settingsOpen = false }
    }
}

// TopBar is the brand and the tools on a root, a way back and a title
// on anything stacked over one.
@Composable
private fun TopBar(model: Model, openSettings: () -> Unit) {
    val root = model.stack.size == 1
    Row(
        Modifier
            .fillMaxWidth()
            .background(Palette.Metal)
            .statusBarsPadding()
            .height(56.dp)
            .padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (root) {
            Spacer(Modifier.width(12.dp))
            Box(
                Modifier
                    .size(26.dp)
                    .clip(CircleShape)
                    .background(Palette.Chrome),
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    Modifier
                        .size(8.dp)
                        .clip(CircleShape)
                        .background(Palette.Accent),
                )
            }
            Text(
                "Music",
                Modifier
                    .padding(start = 10.dp)
                    .weight(1f),
                fontSize = 20.sp,
                fontWeight = FontWeight.SemiBold,
                letterSpacing = (-0.3).sp,
            )
            IconButton(onClick = { model.go(Screen.Search) }) {
                Icon(Glyph.Search, "Search", Modifier.size(22.dp))
            }
            IconButton(onClick = openSettings) { Icon(Glyph.Tune, "Settings", Modifier.size(22.dp)) }
        } else {
            IconButton(onClick = { model.back() }) { Icon(Glyph.Back, "Back", Modifier.size(22.dp)) }
            val title = when (val s = model.screen) {
                is Screen.ArtistPage -> s.artist
                is Screen.AlbumPage -> s.album
                Screen.Search -> "Search"
                is Screen.PlaylistPage -> model.opened[s.id]?.name ?: model.playlists?.find { it.id == s.id }?.name.orEmpty()
                Screen.LibraryMix -> "Library mix"
                else -> ""
            }
            Text(
                title,
                Modifier.weight(1f),
                fontSize = 18.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun NavBar(model: Model) {
    val root = model.stack.first()
    Row(
        Modifier
            .fillMaxWidth()
            .height(60.dp),
    ) {
        NavItem("Home", Glyph.Home, root == Screen.Home) { model.tab(Screen.Home) }
        NavItem("Library", Glyph.Library, root is Screen.Library) { model.tab(Screen.Library()) }
    }
}

@Composable
private fun androidx.compose.foundation.layout.RowScope.NavItem(
    label: String,
    icon: ImageVector,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val tint = if (selected) Palette.Chalk else Palette.Dim
    Column(
        Modifier
            .weight(1f)
            .fillMaxHeight()
            .clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Icon(icon, null, tint = tint, modifier = Modifier.size(22.dp))
        Text(
            label,
            color = tint,
            fontSize = 11.sp,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            modifier = Modifier.padding(top = 3.dp),
        )
    }
}

// position follows the player while it plays, polled, as media3 does
// not report progress on its own; a paused player is read once.
@Composable
private fun rememberPosition(model: Model): Long {
    var position by remember { mutableLongStateOf(model.position()) }
    LaunchedEffect(model.isPlaying, model.playing?.id) {
        position = model.position()
        while (model.isPlaying) {
            delay(500)
            position = model.position()
        }
    }
    return position
}

@Composable
private fun MiniPlayer(model: Model) {
    val now = model.playing ?: return
    val position = rememberPosition(model)
    val progress = if (model.duration > 0) (position.toFloat() / model.duration).coerceIn(0f, 1f) else 0f
    Column {
        Box(
            Modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(Palette.Track),
        ) {
            Box(
                Modifier
                    .fillMaxWidth(progress)
                    .fillMaxHeight()
                    .background(Palette.Accent),
            )
        }
        Row(
            Modifier
                .fillMaxWidth()
                .clickable { model.expanded = true }
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Cover(now.title, now.art, Modifier.size(44.dp), RoundedCornerShape(4.dp))
            Column(
                Modifier
                    .weight(1f)
                    .padding(horizontal = 12.dp),
            ) {
                Text(now.title, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(now.subtitle, color = Palette.Dim, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            PlayButton(model, 40)
            IconButton(onClick = { model.next() }, enabled = model.hasNext) {
                Icon(Glyph.Next, "Next", tint = if (model.hasNext) Palette.Chalk else Palette.Dim)
            }
        }
    }
}

// PlayButton is the one chrome control, as on the web player's bar.
@Composable
private fun PlayButton(model: Model, size: Int) {
    Box(
        Modifier
            .size(size.dp)
            .clip(CircleShape)
            .background(Palette.Chrome)
            .clickable { model.togglePlay() },
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            if (model.isPlaying) Glyph.Pause else Glyph.Play,
            if (model.isPlaying) "Pause" else "Play",
            tint = Palette.OnChrome,
            modifier = Modifier.size((size * 0.5).dp),
        )
    }
}

// PlayerSheet is the full player, over everything: the art, the seek
// rail and the transport, and a buy for a stream.
@Composable
private fun PlayerSheet(model: Model) {
    val now = model.playing ?: return
    val position = rememberPosition(model)
    // While dragging, the rail follows the thumb instead of the player.
    var dragging by remember { mutableStateOf(false) }
    var dragAt by remember { mutableFloatStateOf(0f) }
    val shown = if (dragging) dragAt.toLong() else position
    var confirming by remember { mutableStateOf<YTTrack?>(null) }

    Column(
        Modifier
            .fillMaxSize()
            .background(Palette.Graphite)
            .background(Palette.Metal)
            .clickable(enabled = false) {}
            .systemBarsPadding()
            .padding(horizontal = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .height(56.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = { model.expanded = false }) { Icon(Glyph.Down, "Close the player") }
            Text(
                "NOW PLAYING",
                Modifier.weight(1f),
                color = Palette.Dim,
                fontSize = 11.sp,
                letterSpacing = 1.8.sp,
                fontWeight = FontWeight.SemiBold,
                textAlign = androidx.compose.ui.text.style.TextAlign.Center,
            )
            Spacer(Modifier.width(48.dp))
        }
        Spacer(Modifier.weight(1f))
        Cover(
            now.title,
            now.art,
            Modifier
                .widthIn(max = 420.dp)
                .fillMaxWidth()
                .aspectRatio(1f),
            RoundedCornerShape(10.dp),
        )
        Spacer(Modifier.weight(1f))
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(
                    now.title,
                    fontSize = 22.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(now.subtitle, color = Palette.Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            model.playingStream?.let { t -> BuyButton(model.buyFor(t)) { confirming = t } }
        }
        Spacer(Modifier.height(16.dp))
        Slider(
            value = shown.toFloat().coerceIn(0f, model.duration.toFloat().coerceAtLeast(1f)),
            onValueChange = {
                dragging = true
                dragAt = it
            },
            onValueChangeFinished = {
                model.seekTo(dragAt.toLong())
                dragging = false
            },
            valueRange = 0f..model.duration.toFloat().coerceAtLeast(1f),
            enabled = model.duration > 0,
            colors = SliderDefaults.colors(
                thumbColor = Palette.Chalk,
                activeTrackColor = Palette.Accent,
                inactiveTrackColor = Palette.Track,
            ),
        )
        Row(Modifier.fillMaxWidth()) {
            Text(clock(shown), color = Palette.Dim, fontSize = 12.sp, modifier = Modifier.weight(1f))
            Text(if (model.duration > 0) clock(model.duration) else "–:––", color = Palette.Dim, fontSize = 12.sp)
        }
        Row(
            Modifier
                .fillMaxWidth()
                .padding(vertical = 24.dp),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = { model.previous() }, Modifier.size(56.dp)) {
                Icon(Glyph.Prev, "Previous", Modifier.size(30.dp))
            }
            PlayButton(model, 72)
            IconButton(onClick = { model.next() }, Modifier.size(56.dp), enabled = model.hasNext) {
                Icon(Glyph.Next, "Next", Modifier.size(30.dp), tint = if (model.hasNext) Palette.Chalk else Palette.Dim)
            }
        }
        Spacer(Modifier.height(16.dp))
    }
    BuyDialog(model, confirming) { confirming = null }
}

// Meter is the playing row's three bars, moving while it plays.
@Composable
fun Meter(playing: Boolean) {
    val t = rememberInfiniteTransition(label = "meter")
    Row(
        Modifier.height(14.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        for (i in 0..2) {
            val h by t.animateFloat(
                initialValue = 0.3f,
                targetValue = 1f,
                animationSpec = infiniteRepeatable(tween(420 + i * 130), RepeatMode.Reverse),
                label = "bar$i",
            )
            Box(
                Modifier
                    .width(3.dp)
                    .fillMaxHeight(if (playing) h else 0.35f)
                    .background(Palette.Accent),
            )
        }
    }
}
