package com.jmelahman.music

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.dp

// The look the webapp has: graphite, with chrome for the controls that
// matter and the accent kept for what is playing.
object Palette {
    val Graphite = Color(0xFF0B0B0D)
    val Card = Color(0xFF151518)
    val CardHi = Color(0xFF1E1E22)
    val Edge = Color(0xFF2A2B30)
    val Chalk = Color(0xFFECECEE)
    val Dim = Color(0xFF8E9096)
    val Accent = Color(0xFFF4635E)
    val Owned = Color(0xFF4FD18B)
    val OnChrome = Color(0xFF111214)
    val Track = Color(0x24FFFFFF)

    // The main buttons: flat white on the dark, as the web app's are.
    val Chrome: Brush = SolidColor(Color(0xFFF1F1F3))

    // Brushed graphite, for the bars that frame the screen.
    val Metal = Brush.verticalGradient(
        0f to Color(0xFF232429),
        0.55f to Color(0xFF17181B),
        1f to Color(0xFF121315),
    )

    val Sheen = Brush.linearGradient(
        0.3f to Color.Transparent,
        0.48f to Color(0x17FFFFFF),
        0.62f to Color.Transparent,
    )

    // A placeholder's metal, tinted by the name it stands for so each
    // album's is its own and the same every time.
    fun tinted(name: String): Brush {
        var h = 0
        for (c in name) h = (h * 31 + c.code) % 360
        return Brush.linearGradient(
            listOf(Color.hsl(h.toFloat(), 0.14f, 0.34f), Color.hsl(h.toFloat(), 0.12f, 0.14f)),
        )
    }
}

@Composable
fun MusicTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = Palette.Chalk,
            onPrimary = Palette.Graphite,
            secondary = Palette.Accent,
            background = Palette.Graphite,
            onBackground = Palette.Chalk,
            surface = Palette.Graphite,
            onSurface = Palette.Chalk,
            surfaceVariant = Palette.CardHi,
            onSurfaceVariant = Palette.Dim,
            surfaceContainerHigh = Palette.CardHi,
            outline = Palette.Edge,
            error = Palette.Accent,
        ),
        content = content,
    )
}

// The icons, from the same paths as the webapp's sprite. Filled ones are
// shapes; stroked ones are lines, drawn round.
object Glyph {
    private fun icon(name: String, vararg paths: String, stroke: Float = 0f): ImageVector {
        val b = ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f)
        for (d in paths) {
            if (stroke > 0f) {
                b.addPath(
                    addPathNodes(d),
                    stroke = SolidColor(Color.Black),
                    strokeLineWidth = stroke,
                    strokeLineCap = StrokeCap.Round,
                    strokeLineJoin = StrokeJoin.Round,
                )
            } else {
                b.addPath(addPathNodes(d), fill = SolidColor(Color.Black))
            }
        }
        return b.build()
    }

    val Play = icon("play", "M8 5.5v13l11-6.5z")
    val Pause = icon("pause", "M7 5h3.5v14H7zM13.5 5H17v14h-3.5z")
    val Prev = icon("prev", "M6 5h2v14H6zM9.5 12 19 5.5v13z")
    val Next = icon("next", "M16 5h2v14h-2zM14.5 12 5 18.5v-13z")
    val Shuffle = icon("shuffle", "M16 4h4v4M4 20 20 4M20 16v4h-4M15 15l5 5M4 4l5 5", stroke = 1.8f)
    val Search = icon("search", "M10.5 4a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13zM15.5 15.5 20 20", stroke = 1.8f)
    val Save = icon("save", "M12 4v11m-4.5-4.5L12 15l4.5-4.5M5 19.5h14", stroke = 1.8f)
    val Saved = icon(
        "saved",
        "M12 3.5a8.5 8.5 0 1 0 0 17 8.5 8.5 0 0 0 0-17zm-1.2 12.3-3.6-3.6 1.3-1.3 2.3 2.3 4.9-4.9 1.3 1.3z",
    )
    val Mix = icon("mix", "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zm0 6.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5z")
    val Note = icon(
        "note",
        "M9 18V5l10-2v13M9 18a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0zm10-2a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z",
        stroke = 1.6f,
    )
    val Home = icon("home", "M4 10.5 12 4l8 6.5V20h-5.5v-6h-5v6H4z")
    val Library = icon("library", "M4 5h2.2v14H4zM8.4 5h2.2v14H8.4zM12.6 5.7l2-.7 4.8 13.3-2 .7z")
    val Tune = icon(
        "tune",
        "M4 7h9M19 7h1M4 17h3M13 17h7",
        "M16 4.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5zM10 14.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5z",
        stroke = 1.7f,
    )
    val Back = icon("back", "M15 5l-7 7 7 7", stroke = 2f)
    val Down = icon("down", "M6 9l6 6 6-6", stroke = 2f)
    val Close = icon("close", "M6 6l12 12M18 6 6 18", stroke = 2f)
    val Update = icon("update", "M12 16V5m-4.5 4.5L12 5l4.5 4.5M5 19.5h14", stroke = 1.8f)
    val More = icon(
        "more",
        "M12 5.5a1.6 1.6 0 1 0 0 3.2 1.6 1.6 0 0 0 0-3.2zm0 5a1.6 1.6 0 1 0 0 3.2 1.6 1.6 0 0 0 0-3.2zm0 5a1.6 1.6 0 1 0 0 3.2 1.6 1.6 0 0 0 0-3.2z",
    )
    val Plus = icon("plus", "M12 5v14M5 12h14", stroke = 1.8f)
    val List = icon("list", "M4 6h12M4 11h12M4 16h7M18 9v8.5", stroke = 1.8f)
    val Smart = icon(
        "smart",
        "M11 3l1.9 5.1L18 10l-5.1 1.9L11 17l-1.9-5.1L4 10l5.1-1.9zM18 14l.9 2.1L21 17l-2.1.9L18 20l-.9-2.1L15 17l2.1-.9z",
    )
}
