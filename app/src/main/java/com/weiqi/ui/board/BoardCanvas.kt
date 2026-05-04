package com.weiqi.ui.board

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import com.weiqi.engine.Board
import com.weiqi.engine.CellState
import com.weiqi.engine.Point
import com.weiqi.ui.theme.Zen
import kotlin.math.min
import kotlin.math.roundToInt

data class BoardOverlay(
    val lastMove: Point? = null,
    val koPoint: Point? = null,
    val deadStones: Set<Point> = emptySet(),
    val territoryBlack: Set<Point> = emptySet(),
    val territoryWhite: Set<Point> = emptySet(),
    val labels: Map<Point, String> = emptyMap()
)

/**
 * Premium kaya-board renderer.
 *
 * Layers (back to front):
 *   1. Table backdrop with soft drop shadow.
 *   2. Kaya wood gradient + edge bevel.
 *   3. 20%-opacity ink grid + hoshi stars.
 *   4. Territory overlays.
 *   5. Stones with subtle radial shading.
 *   6. Last-move ring, ko marker, dead-stone X, optional labels.
 */
@Composable
fun BoardCanvas(
    board: Board,
    overlay: BoardOverlay = BoardOverlay(),
    onTap: (Point) -> Unit = {},
    modifier: Modifier = Modifier,
    showCoordinates: Boolean = false
) {
    val starPoints = remember(board.size) { starPointsFor(board.size) }

    Canvas(
        modifier = modifier
            .fillMaxSize()
            .pointerInput(board.size) {
                detectTapGestures { offset ->
                    val side = min(size.width, size.height).toFloat()
                    val pad = side * 0.06f
                    val usable = side - 2 * pad
                    val step = usable / (board.size - 1)
                    val col = ((offset.x - pad) / step).roundToInt()
                    val row = ((offset.y - pad) / step).roundToInt()
                    if (row in 0 until board.size && col in 0 until board.size) {
                        onTap(Point(row, col))
                    }
                }
            }
    ) {
        val side = min(size.width, size.height)
        val pad = side * 0.06f
        val usable = side - 2 * pad
        val step = usable / (board.size - 1)
        val stoneR = step * 0.46f
        val ink20 = Zen.gridInk.copy(alpha = 0.20f)
        val ink70 = Zen.gridInk.copy(alpha = 0.70f)

        // 1. Soft table shadow under the board.
        val shadowInset = side * 0.012f
        drawRoundRectShadow(
            topLeft = Offset(shadowInset, shadowInset * 1.4f),
            sz = Size(side - 2 * shadowInset, side - 2 * shadowInset),
            color = Zen.tableShadow
        )

        // 2. Kaya wood: vertical gradient + slight edge bevel.
        drawRect(
            brush = Brush.verticalGradient(
                0f to Zen.kayaWood,
                1f to Zen.kayaWoodEdge
            ),
            size = Size(side, side)
        )
        // Inner darken band to suggest a wooden frame.
        drawRect(
            color = Zen.kayaWoodEdge.copy(alpha = 0.35f),
            topLeft = Offset(0f, 0f),
            size = Size(side, side),
            style = Stroke(width = side * 0.012f)
        )

        // 3. Grid lines (1dp ink at 20%).
        for (i in 0 until board.size) {
            val pos = pad + step * i
            drawLine(ink20, Offset(pad, pos), Offset(pad + usable, pos), strokeWidth = 1.2f)
            drawLine(ink20, Offset(pos, pad), Offset(pos, pad + usable), strokeWidth = 1.2f)
        }
        // Outer rectangle slightly darker.
        drawRect(
            color = ink20.copy(alpha = 0.30f),
            topLeft = Offset(pad, pad),
            size = Size(usable, usable),
            style = Stroke(width = 1.4f)
        )
        // Hoshi.
        for (sp in starPoints) {
            drawCircle(ink70, radius = step * 0.085f,
                center = Offset(pad + step * sp.col, pad + step * sp.row))
        }

        // 4. Territory tint dots.
        for (p in overlay.territoryBlack) {
            drawCircle(Color.Black.copy(alpha = 0.22f), radius = stoneR * 0.40f,
                center = Offset(pad + step * p.col, pad + step * p.row))
        }
        for (p in overlay.territoryWhite) {
            drawCircle(Color.White.copy(alpha = 0.55f), radius = stoneR * 0.40f,
                center = Offset(pad + step * p.col, pad + step * p.row))
        }

        // 5. Stones.
        for (r in 0 until board.size) for (c in 0 until board.size) {
            val state = board.get(r, c)
            if (state == CellState.EMPTY) continue
            val center = Offset(pad + step * c, pad + step * r)
            // Tight ambient shadow.
            drawCircle(
                color = Color.Black.copy(alpha = 0.25f),
                radius = stoneR,
                center = center.copy(x = center.x + stoneR * 0.05f, y = center.y + stoneR * 0.18f)
            )
            // Stone body — radial gradient for shell/ink shading.
            val isBlack = state == CellState.BLACK
            val highlight = if (isBlack) Zen.blackStoneTop else Zen.whiteStoneTop
            val body = if (isBlack) Zen.blackStoneBottom else Zen.whiteStoneBottom
            val shadeCenter = Offset(center.x - stoneR * 0.30f, center.y - stoneR * 0.30f)
            drawCircle(
                brush = Brush.radialGradient(
                    0f to highlight,
                    1f to body,
                    center = shadeCenter,
                    radius = stoneR * 1.4f
                ),
                radius = stoneR,
                center = center
            )
            // Hairline outline.
            drawCircle(
                color = if (isBlack) Color.Black else Zen.gridInk.copy(alpha = 0.55f),
                radius = stoneR,
                center = center,
                style = Stroke(width = if (isBlack) 0.6f else 1.0f)
            )
            if (Point(r, c) in overlay.deadStones) {
                val s = stoneR * 0.55f
                val x = Color(0xFFB23A2E)
                drawLine(x, Offset(center.x - s, center.y - s), Offset(center.x + s, center.y + s), strokeWidth = 3f)
                drawLine(x, Offset(center.x - s, center.y + s), Offset(center.x + s, center.y - s), strokeWidth = 3f)
            }
        }

        // 6. Last-move ring.
        overlay.lastMove?.let { p ->
            val state = board.get(p)
            val ringColor = if (state == CellState.BLACK) Color.White else Color.Black
            drawCircle(
                color = ringColor,
                radius = stoneR * 0.36f,
                center = Offset(pad + step * p.col, pad + step * p.row),
                style = Stroke(width = 2.2f)
            )
        }
        // Ko marker.
        overlay.koPoint?.let { p ->
            drawCircle(
                color = Color(0xFFB23A2E),
                radius = stoneR * 0.28f,
                center = Offset(pad + step * p.col, pad + step * p.row),
                style = Stroke(width = 2f)
            )
        }
    }
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawRoundRectShadow(
    topLeft: Offset, sz: Size, color: Color
) {
    // Cheap soft-shadow approximation: stack 3 progressively larger translucent rects.
    val steps = 3
    for (i in 0 until steps) {
        val grow = (i + 1) * 4f
        drawRect(
            color = color.copy(alpha = color.alpha / (i + 2f)),
            topLeft = Offset(topLeft.x - grow, topLeft.y - grow + 2f),
            size = Size(sz.width + grow * 2, sz.height + grow * 2)
        )
    }
}

private fun starPointsFor(size: Int): List<Point> {
    if (size != 9 && size != 13 && size != 19) return emptyList()
    val edge = if (size == 9) 2 else 3
    val far = size - 1 - edge
    val mid = size / 2
    return when (size) {
        9 -> listOf(Point(edge, edge), Point(edge, far), Point(far, edge), Point(far, far), Point(mid, mid))
        13 -> listOf(Point(edge, edge), Point(edge, far), Point(far, edge), Point(far, far), Point(mid, mid))
        else -> listOf(
            Point(edge, edge), Point(edge, mid), Point(edge, far),
            Point(mid, edge), Point(mid, mid), Point(mid, far),
            Point(far, edge), Point(far, mid), Point(far, far)
        )
    }
}
