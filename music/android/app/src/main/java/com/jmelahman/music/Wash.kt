package com.jmelahman.music

import android.os.Build
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.BlurredEdgeTreatment
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.min
import androidx.core.graphics.ColorUtils
import androidx.core.graphics.drawable.toBitmap
import coil.imageLoader
import coil.request.ImageRequest
import coil.request.SuccessResult
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

// The wash behind the top of the screen, as the web app's: clouds in the
// colour of the cover in view, or of what is playing, with neighbouring
// hues drifting through them like oil on water. With no cover it is a
// quiet slate. It swells below the top bar and fades into the page.

private val Slate = Color(0xFF323949)

// Each cloud: where it sits and how wide and tall it reaches, as shares
// of a layer half again the wash's size, and how its colour turns from
// the cover's: a hue shift, and whether it is a touch lighter.
private class Cloud(val x: Float, val y: Float, val rx: Float, val ry: Float, val hue: Float, val lift: Float)

private val clouds = listOf(
    Cloud(0.16f, 0.34f, 0.48f, 0.62f, 0f, 0f),
    Cloud(0.56f, 0.22f, 0.42f, 0.56f, 55f, 0f),
    Cloud(0.88f, 0.40f, 0.50f, 0.66f, -50f, 0f),
    Cloud(0.40f, 0.62f, 0.56f, 0.48f, 140f, 0.05f),
)

// glows remembers each cover's colour, null when it has none to give.
private val glows = HashMap<String, Color?>()

@Composable
fun Wash(src: String?, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    var glow by remember { mutableStateOf<Color?>(null) }
    LaunchedEffect(src) {
        glow = when {
            src.isNullOrEmpty() -> null
            glows.containsKey(src) -> glows[src]
            else -> coverColour(context, src).also { glows[src] = it }
        }
    }
    val wash by animateColorAsState(glow ?: Slate, tween(900), label = "wash")

    // The layer drifts as the web's does: across, then back, over 48s.
    val t by rememberInfiniteTransition(label = "drift").animateFloat(
        0f,
        1f,
        infiniteRepeatable(tween(48_000, easing = LinearEasing), RepeatMode.Reverse),
        label = "drift",
    )

    BoxWithConstraints(modifier.fillMaxWidth()) {
        Box(
            Modifier
                .fillMaxWidth()
                .height(min(640.dp, maxHeight * 0.8f))
                .graphicsLayer {
                    alpha = 0.55f
                    compositingStrategy = CompositingStrategy.Offscreen
                }
                // Blur is Android 12's; before it the clouds' own soft
                // edges carry the look.
                .then(if (Build.VERSION.SDK_INT >= 31) Modifier.blur(36.dp, BlurredEdgeTreatment.Unbounded) else Modifier)
                .drawWithContent {
                    drawClouds(wash, t)
                    // Faint at the top, fullest a little way down, gone by
                    // the bottom.
                    drawRect(
                        Brush.verticalGradient(
                            0f to Color.Black.copy(alpha = 0.25f),
                            0.28f to Color.Black.copy(alpha = 0.7f),
                            0.55f to Color.Black.copy(alpha = 0.35f),
                            1f to Color.Transparent,
                        ),
                        blendMode = BlendMode.DstIn,
                    )
                },
        )
    }
}

private fun DrawScope.drawClouds(wash: Color, t: Float) {
    val w = size.width
    val h = size.height
    // The layer is 150% of the wash, slid by the drift: 0%→60%→100%
    // across and 20%→0%→40% down, as the web's keyframes go.
    val px = if (t < 0.5f) t / 0.5f * 0.6f else 0.6f + (t - 0.5f) / 0.5f * 0.4f
    val py = if (t < 0.5f) 0.2f - t / 0.5f * 0.2f else (t - 0.5f) / 0.5f * 0.4f
    val lw = w * 1.5f
    val lh = h * 1.5f
    val ox = -(lw - w) * px
    val oy = -(lh - h) * py
    for (c in clouds) {
        val colour = turn(wash, c.hue, c.lift)
        val centre = Offset(ox + lw * c.x, oy + lh * c.y)
        val rx = lw * c.rx
        val ry = lh * c.ry
        // An ellipse: a circle's gradient, squashed about its centre.
        scale(1f, ry / rx, centre) {
            drawCircle(
                Brush.radialGradient(
                    0f to colour,
                    0.7f to colour.copy(alpha = 0f),
                    center = centre,
                    radius = rx,
                ),
                radius = rx,
                center = centre,
            )
        }
    }
}

// turn shifts a colour's hue, keeps it from going grey, and lifts it.
private fun turn(c: Color, hue: Float, lift: Float): Color {
    if (hue == 0f && lift == 0f) return c
    val hsl = FloatArray(3)
    ColorUtils.colorToHSL(c.toArgb(), hsl)
    hsl[0] = ((hsl[0] + hue) % 360f + 360f) % 360f
    hsl[1] = maxOf(hsl[1], 0.3f)
    hsl[2] = (hsl[2] + lift).coerceAtMost(1f)
    return Color(ColorUtils.HSLToColor(hsl))
}

// coverColour is a cover's pixels averaged, the more colourful ones
// counting most, then made vivid enough to show; null without a cover.
private suspend fun coverColour(context: android.content.Context, src: String): Color? {
    val request = ImageRequest.Builder(context).data(src).size(24).allowHardware(false).build()
    val result = context.imageLoader.execute(request) as? SuccessResult ?: return null
    return withContext(Dispatchers.Default) {
        val bitmap = result.drawable.toBitmap(24, 24)
        var r = 0.0
        var g = 0.0
        var b = 0.0
        var weight = 0.0
        for (y in 0 until bitmap.height) {
            for (x in 0 until bitmap.width) {
                val p = bitmap.getPixel(x, y)
                val pr = (p shr 16) and 0xFF
                val pg = (p shr 8) and 0xFF
                val pb = p and 0xFF
                val wt = 0.05 + (maxOf(pr, pg, pb) - minOf(pr, pg, pb)) / 255.0
                r += pr * wt
                g += pg * wt
                b += pb * wt
                weight += wt
            }
        }
        vivid((r / weight).toInt(), (g / weight).toInt(), (b / weight).toInt())
    }
}

// vivid lifts a colour's saturation and sets its lightness into a band
// that shows against the page without glaring.
private fun vivid(r: Int, g: Int, b: Int): Color {
    val hsl = FloatArray(3)
    ColorUtils.RGBToHSL(r, g, b, hsl)
    hsl[1] = minOf(1f, hsl[1] * 1.7f)
    hsl[2] = hsl[2].coerceIn(0.4f, 0.55f)
    return Color(ColorUtils.HSLToColor(hsl))
}
