package com.weiqi.ui.board

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.weiqi.engine.StoneColor
import com.weiqi.ui.theme.Zen

/** Small standalone stone, e.g. for player avatars in lists. */
@Composable
fun MiniStone(color: StoneColor, modifier: Modifier = Modifier, size: Dp = 24.dp) {
    Canvas(modifier = modifier.size(size)) {
        val r = this.size.minDimension / 2f
        val center = Offset(this.size.width / 2f, this.size.height / 2f)
        val isBlack = color == StoneColor.BLACK
        val top = if (isBlack) Zen.blackStoneTop else Zen.whiteStoneTop
        val bot = if (isBlack) Zen.blackStoneBottom else Zen.whiteStoneBottom
        // Ambient shadow.
        drawCircle(Color.Black.copy(alpha = 0.20f), radius = r,
            center = center.copy(y = center.y + r * 0.18f))
        drawCircle(
            brush = Brush.radialGradient(
                0f to top, 1f to bot,
                center = Offset(center.x - r * 0.3f, center.y - r * 0.3f),
                radius = r * 1.4f
            ),
            radius = r, center = center
        )
        drawCircle(
            color = if (isBlack) Color.Black else Zen.gridInk.copy(alpha = 0.55f),
            radius = r, center = center,
            style = Stroke(width = if (isBlack) 0.6f else 1f)
        )
    }
}
