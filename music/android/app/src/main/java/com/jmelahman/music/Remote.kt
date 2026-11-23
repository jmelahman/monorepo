package com.jmelahman.music

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// The phone as a remote for the web player: a button that picks what
// plays, the phone or a page open in a browser, and the page's queue to
// jump about in.

// DevicesButton is there while a web player is open, lit while the phone
// drives one.
@Composable
fun DevicesButton(model: Model) {
    if (model.remotePlayers.isEmpty() && model.remoteId == null) return
    var open by remember { mutableStateOf(false) }
    val on = model.remoteId != null
    IconButton(onClick = { open = true }) {
        Icon(
            Glyph.Speaker,
            if (on) "Playing on ${model.remote?.name.orEmpty()}" else "Play on…",
            Modifier.size(22.dp),
            tint = if (on) Palette.Accent else Palette.Chalk,
        )
    }
    if (open) DevicesDialog(model) { open = false }
}

@Composable
private fun DevicesDialog(model: Model, dismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = dismiss,
        containerColor = Palette.CardHi,
        title = { Text("Play on") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                DeviceRow("This phone", null, model.remoteId == null) {
                    model.useRemote(null)
                    dismiss()
                }
                for (p in model.remotePlayers) {
                    val s = p.state
                    val what = when {
                        s == null -> "Nothing playing"
                        s.playing -> "Playing ${s.title}"
                        else -> "Paused on ${s.title}"
                    }
                    DeviceRow(p.name, what, model.remoteId == p.id) {
                        model.useRemote(p.id)
                        dismiss()
                    }
                }
            }
        },
        confirmButton = { TextButton(onClick = dismiss) { Text("Done") } },
    )
}

@Composable
private fun DeviceRow(name: String, sub: String?, on: Boolean, pick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = pick)
            .padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(name, fontWeight = FontWeight.SemiBold, color = if (on) Palette.Accent else Palette.Chalk)
            if (sub != null) Text(sub, color = Palette.Dim, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        if (on) Icon(Glyph.Saved, "Selected", Modifier.size(20.dp), tint = Palette.Accent)
    }
}

// UpNextButton lists what the web player plays next; a tap plays from
// there. The phone's own queue isn't listed, so it is the web's alone.
@Composable
fun UpNextButton(model: Model) {
    val upNext = model.remote?.state?.upNext ?: return
    var open by remember { mutableStateOf(false) }
    IconButton(onClick = { open = true }) { Icon(Glyph.List, "Up next", Modifier.size(22.dp)) }
    if (!open) return
    AlertDialog(
        onDismissRequest = { open = false },
        containerColor = Palette.CardHi,
        title = { Text("Up next") },
        text = {
            if (upNext.isEmpty()) {
                Text("Nothing after this.", color = Palette.Dim)
            } else {
                LazyColumn(Modifier.heightIn(max = 480.dp)) {
                    items(upNext, key = { it.index }) { t ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .clickable {
                                    model.jump(t.index)
                                    open = false
                                }
                                .padding(vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Cover(t.title, t.art.ifEmpty { null }, Modifier.size(40.dp), RoundedCornerShape(4.dp))
                            Column(
                                Modifier
                                    .weight(1f)
                                    .padding(start = 12.dp),
                            ) {
                                Text(t.title, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(t.subtitle, color = Palette.Dim, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(onClick = { open = false }) { Text("Close") } },
    )
}
