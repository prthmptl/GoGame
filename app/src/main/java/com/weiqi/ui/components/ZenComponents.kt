package com.weiqi.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

/** Paper-tone card. Soft 14dp rounding for the "carved wood" feel. */
@Composable
fun ZenCard(
    modifier: Modifier = Modifier,
    container: Color = MaterialTheme.colorScheme.surfaceContainer,
    contentPadding: PaddingValues = PaddingValues(16.dp),
    content: @Composable () -> Unit
) {
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(14.dp))
            .background(container)
            .padding(contentPadding)
    ) { content() }
}

/**
 * Pill-shaped tag. Picks a contrasting text color automatically based on the container.
 */
@Composable
fun ZenChip(
    text: String,
    modifier: Modifier = Modifier,
    container: Color? = null,
    contentColor: Color? = null
) {
    val bg = container ?: MaterialTheme.colorScheme.secondaryContainer
    val fg = contentColor ?: contentColorFor(bg)
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(999.dp))
            .background(bg)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            color = fg
        )
    }
}

private fun contentColorFor(bg: Color): Color {
    // Simple luminance check — dark backgrounds get light text.
    val l = 0.299 * bg.red + 0.587 * bg.green + 0.114 * bg.blue
    return if (l < 0.5) Color(0xFFFFF8F3) else Color(0xFF221A10)
}
