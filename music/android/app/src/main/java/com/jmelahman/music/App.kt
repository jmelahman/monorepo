package com.jmelahman.music

import android.Manifest
import android.os.Build
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
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
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
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
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.media3.common.Player
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
        BackHandler(enabled = model.expanded || model.stack != listOf(Screen.Home)) { model.back() }
        var settingsOpen by remember { mutableStateOf(false) }
        // Saving to the phone says how it is going in a notification, which
        // Android 13 on asks for first: once, when something is first kept.
        val ask = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {}
        val keeping = model.kept.isNotEmpty()
        LaunchedEffect(keeping) {
            if (keeping && Build.VERSION.SDK_INT >= 33) ask.launch(Manifest.permission.POST_NOTIFICATIONS)
        }

        Box(
            Modifier
                .fillMaxSize()
                .background(Palette.Graphite),
        ) {
            Wash(model.backdrop())
            Scaffold(
                containerColor = Color.Transparent,
                // A clear scaffold can't pick its text colour from its own.
                contentColor = Palette.Chalk,
                contentWindowInsets = WindowInsets(0),
                topBar = { TopBar(model) { settingsOpen = true } },
                bottomBar = {
                    Column(
                        Modifier
                            .background(Palette.Metal)
                            .navigationBarsPadding(),
                    ) {
                        if (model.playing != null) MiniPlayer(model)
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

// PurchasesButton opens the web app's purchases page, counting the buys
// that failed and need a hand.
@Composable
private fun PurchasesButton(model: Model) {
    val failed = model.failedBuys
    IconButton(onClick = { model.openPurchases() }) {
        BadgedBox(badge = {
            if (failed > 0) {
                Badge(containerColor = Palette.Accent) { Text(if (failed > 9) "9+" else "$failed") }
            }
        }) {
            Icon(Glyph.Bag, if (failed > 0) "Purchases: $failed failed" else "Purchases", Modifier.size(22.dp))
        }
    }
}

// TopBar is the brand, which goes home, and the tools on a root, a way
// back and a title on anything stacked over one.
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
            Row(
                Modifier
                    .weight(1f)
                    .fillMaxHeight()
                    .clickable { model.tab(Screen.Home) }
                    .padding(start = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
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
                    Modifier.padding(start = 10.dp),
                    fontSize = 20.sp,
                    fontWeight = FontWeight.SemiBold,
                    letterSpacing = (-0.3).sp,
                )
            }
            IconButton(onClick = { model.tab(Screen.Library()) }) {
                Icon(
                    Glyph.Library,
                    "Library",
                    Modifier.size(22.dp),
                    tint = if (model.screen is Screen.Library) Palette.Chalk else Palette.Dim,
                )
            }
            IconButton(onClick = { model.go(Screen.Search) }) {
                Icon(Glyph.Search, "Search", Modifier.size(22.dp))
            }
            if (model.buysReady) PurchasesButton(model)
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

// position follows the player while it plays, polled, as media3 does
// not report progress on its own; a paused player is read once.
@Composable
private fun rememberPosition(model: Model): Long {
    var position by remember { mutableLongStateOf(model.position()) }
    LaunchedEffect(model.isPlaying, model.playing?.id, model.seeks) {
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
    var confirming by remember { mutableStateOf<YTTrack?>(null) }

    // The player has the wash of what is playing, as the screens do.
    Box(
        Modifier
            .fillMaxSize()
            .background(Palette.Graphite)
            .clickable(enabled = false) {},
    ) {
        Wash(now.art)
        Column(
            Modifier
                .fillMaxSize()
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
                model.playingStream?.let { t -> BuyButton(model.buyFor(t), model.owns(t)) { confirming = t } }
            }
            Spacer(Modifier.height(16.dp))
            SeekBar(position, model.duration, model::seekTo)
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(vertical = 24.dp),
                horizontalArrangement = Arrangement.SpaceEvenly,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                ModeButton(Glyph.Shuffle, if (model.shuffle) "Shuffle is on" else "Shuffle", model.shuffle) {
                    model.toggleShuffle()
                }
                IconButton(onClick = { model.previous() }, Modifier.size(56.dp)) {
                    Icon(Glyph.Prev, "Previous", Modifier.size(30.dp))
                }
                PlayButton(model, 72)
                IconButton(onClick = { model.next() }, Modifier.size(56.dp), enabled = model.hasNext) {
                    Icon(Glyph.Next, "Next", Modifier.size(30.dp), tint = if (model.hasNext) Palette.Chalk else Palette.Dim)
                }
                ModeButton(
                    if (model.repeat == Player.REPEAT_MODE_ONE) Glyph.RepeatOne else Glyph.Repeat,
                    when (model.repeat) {
                        Player.REPEAT_MODE_ALL -> "Repeating the queue"
                        Player.REPEAT_MODE_ONE -> "Repeating this track"
                        else -> "Repeat"
                    },
                    model.repeat != Player.REPEAT_MODE_OFF,
                ) { model.cycleRepeat() }
            }
            Spacer(Modifier.height(16.dp))
        }
    }
    BuyDialog(model, confirming) { confirming = null }
}

// ModeButton is shuffle or repeat: the accent with a dot under it while
// on, dim while off.
@Composable
private fun ModeButton(glyph: ImageVector, label: String, on: Boolean, onClick: () -> Unit) {
    IconButton(onClick = onClick, Modifier.size(48.dp)) {
        Box(contentAlignment = Alignment.Center) {
            Icon(glyph, label, Modifier.size(22.dp), tint = if (on) Palette.Accent else Palette.Dim)
            if (on) {
                Box(
                    Modifier
                        .padding(top = 34.dp)
                        .size(4.dp)
                        .clip(CircleShape)
                        .background(Palette.Accent),
                )
            }
        }
    }
}

// SeekBar is the player's rail: a hairline that thickens under the finger
// and shows a knob while held. A tap jumps; a drag scrubs, and the player
// only seeks when the finger lifts. The clocks read the elapsed and the
// time left, and brighten while scrubbing.
@Composable
private fun SeekBar(position: Long, duration: Long, onSeek: (Long) -> Unit) {
    var held by remember { mutableStateOf(false) }
    var heldAt by remember { mutableFloatStateOf(0f) }
    val enabled = duration > 0
    val fraction = when {
        held -> heldAt
        enabled -> (position.toFloat() / duration).coerceIn(0f, 1f)
        else -> 0f
    }
    val grow by animateFloatAsState(if (held) 1f else 0f, tween(140), label = "seek")
    val length by rememberUpdatedState(duration)
    val seek by rememberUpdatedState(onSeek)
    val shown = if (held) (heldAt * duration).toLong() else position
    Column(Modifier.fillMaxWidth()) {
        Canvas(
            Modifier
                .fillMaxWidth()
                .height(32.dp)
                .pointerInput(enabled) {
                    if (!enabled) return@pointerInput
                    fun at(x: Float) = (x / size.width).coerceIn(0f, 1f)
                    awaitEachGesture {
                        val down = awaitFirstDown()
                        down.consume()
                        held = true
                        heldAt = at(down.position.x)
                        while (true) {
                            val c = awaitPointerEvent().changes.firstOrNull { it.id == down.id } ?: break
                            if (!c.pressed) break
                            heldAt = at(c.position.x)
                            c.consume()
                        }
                        seek((heldAt * length).toLong())
                        held = false
                    }
                },
        ) {
            val h = (3f + 4f * grow).dp.toPx()
            val y = size.height / 2
            val corner = CornerRadius(h / 2)
            drawRoundRect(Palette.Track, Offset(0f, y - h / 2), Size(size.width, h), corner)
            val x = size.width * fraction
            if (x > 0f) {
                drawRoundRect(
                    Brush.horizontalGradient(listOf(Palette.Accent.copy(alpha = 0.55f), Palette.Accent), endX = x),
                    Offset(0f, y - h / 2),
                    Size(x, h),
                    corner,
                )
            }
            if (grow > 0f) {
                drawCircle(Palette.Accent.copy(alpha = 0.25f * grow), 16.dp.toPx() * grow, Offset(x, y))
                drawCircle(Palette.Chalk, 7.dp.toPx() * grow, Offset(x, y))
            }
        }
        Row(Modifier.fillMaxWidth()) {
            val tint = if (held) Palette.Chalk else Palette.Dim
            val digits = TextStyle(fontSize = 12.sp, fontFeatureSettings = "tnum")
            Text(clock(shown), Modifier.weight(1f), color = tint, style = digits)
            Text(
                if (enabled) "−" + clock((duration - shown).coerceAtLeast(0)) else "–:––",
                color = tint,
                style = digits,
            )
        }
    }
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
