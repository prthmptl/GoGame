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
import androidx.compose.ui.text.TextMeasurer
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.sp
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
    val moveNumbers: Map<Point, Int> = emptyMap()
)

data class BoardAppearance(
    val board: Color = Zen.kayaWood,
    val boardEdge: Color = Zen.kayaWoodEdge,
    val ink: Color = Zen.gridInk,
    val showCoordinates: Boolean = false
) {
    companion object {
        val ClassicWood = BoardAppearance()
        val MinimalPaper = BoardAppearance(
            board = Color(0xFFFAF6EE),
            boardEdge = Color(0xFFE0DAC8),
            ink = Color(0xFF1A1A1A)
        )
        // "Slate": light cool-grey board with dark ink — keeps black stones visible
        // while feeling distinctly cooler/quieter than the warm wood themes.
        val DarkSlate = BoardAppearance(
            board = Color(0xFFD8D7D2),
            boardEdge = Color(0xFFB7B6B0),
            ink = Color(0xFF1A1A1A)
        )
        val HighContrast = BoardAppearance(
            board = Color(0xFFFFF1CD),
            boardEdge = Color(0xFFE6D496),
            ink = Color(0xFF000000)
        )
    }
}

@Composable
fun BoardCanvas(
    board: Board,
    overlay: BoardOverlay = BoardOverlay(),
    appearance: BoardAppearance = BoardAppearance.ClassicWood,
    onTap: (Point) -> Unit = {},
    modifier: Modifier = Modifier
) {
    val starPoints = remember(board.size) { starPointsFor(board.size) }
    val measurer: TextMeasurer = rememberTextMeasurer()

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
        val pad = side * (if (appearance.showCoordinates) 0.085f else 0.06f)
        val usable = side - 2 * pad
        val step = usable / (board.size - 1)
        val stoneR = step * 0.46f
        val ink20 = appearance.ink.copy(alpha = 0.20f)
        val ink70 = appearance.ink.copy(alpha = 0.70f)

        // Soft shadow under the board.
        val shadowInset = side * 0.012f
        drawSoftShadow(Offset(shadowInset, shadowInset * 1.4f),
            Size(side - 2 * shadowInset, side - 2 * shadowInset))

        // Board fill.
        drawRect(
            brush = Brush.verticalGradient(0f to appearance.board, 1f to appearance.boardEdge),
            size = Size(side, side)
        )
        drawRect(
            color = appearance.boardEdge.copy(alpha = 0.35f),
            size = Size(side, side),
            style = Stroke(width = side * 0.012f)
        )

        // Grid lines.
        for (i in 0 until board.size) {
            val pos = pad + step * i
            drawLine(ink20, Offset(pad, pos), Offset(pad + usable, pos), strokeWidth = 1.2f)
            drawLine(ink20, Offset(pos, pad), Offset(pos, pad + usable), strokeWidth = 1.2f)
        }
        drawRect(
            color = ink20.copy(alpha = 0.30f),
            topLeft = Offset(pad, pad),
            size = Size(usable, usable),
            style = Stroke(width = 1.4f)
        )
        for (sp in starPoints) {
            drawCircle(ink70, radius = step * 0.085f,
                center = Offset(pad + step * sp.col, pad + step * sp.row))
        }

        // Coordinates labels (Western: A–T excluding I; rows = N..1 from top to bottom).
        if (appearance.showCoordinates) {
            val labelStyle = androidx.compose.ui.text.TextStyle(
                color = appearance.ink.copy(alpha = 0.65f),
                fontSize = (step * 0.32f).coerceAtMost(20f).sp
            )
            for (i in 0 until board.size) {
                val colLetter = colLetter(i)
                val rowLabel = (board.size - i).toString()
                val xCol = pad + step * i
                val yRow = pad + step * i
                val colLayout = measurer.measure(colLetter, labelStyle)
                val rowLayout = measurer.measure(rowLabel, labelStyle)
                drawText(colLayout, topLeft = Offset(
                    xCol - colLayout.size.width / 2f,
                    pad - colLayout.size.height - step * 0.10f
                ))
                drawText(colLayout, topLeft = Offset(
                    xCol - colLayout.size.width / 2f,
                    pad + usable + step * 0.10f
                ))
                drawText(rowLayout, topLeft = Offset(
                    pad - rowLayout.size.width - step * 0.18f,
                    yRow - rowLayout.size.height / 2f
                ))
                drawText(rowLayout, topLeft = Offset(
                    pad + usable + step * 0.18f,
                    yRow - rowLayout.size.height / 2f
                ))
            }
        }

        // Territory tint.
        for (p in overlay.territoryBlack) {
            drawCircle(Color.Black.copy(alpha = 0.22f), radius = stoneR * 0.40f,
                center = Offset(pad + step * p.col, pad + step * p.row))
        }
        for (p in overlay.territoryWhite) {
            drawCircle(Color.White.copy(alpha = 0.55f), radius = stoneR * 0.40f,
                center = Offset(pad + step * p.col, pad + step * p.row))
        }

        // Stones.
        for (r in 0 until board.size) for (c in 0 until board.size) {
            val state = board.get(r, c)
            if (state == CellState.EMPTY) continue
            val center = Offset(pad + step * c, pad + step * r)
            drawCircle(
                color = Color.Black.copy(alpha = 0.25f),
                radius = stoneR,
                center = center.copy(x = center.x + stoneR * 0.05f, y = center.y + stoneR * 0.18f)
            )
            val isBlack = state == CellState.BLACK
            val highlight = if (isBlack) Zen.blackStoneTop else Zen.whiteStoneTop
            val body = if (isBlack) Zen.blackStoneBottom else Zen.whiteStoneBottom
            drawCircle(
                brush = Brush.radialGradient(
                    0f to highlight, 1f to body,
                    center = Offset(center.x - stoneR * 0.30f, center.y - stoneR * 0.30f),
                    radius = stoneR * 1.4f
                ),
                radius = stoneR,
                center = center
            )
            drawCircle(
                color = if (isBlack) Color.Black else Zen.gridInk.copy(alpha = 0.55f),
                radius = stoneR,
                center = center,
                style = Stroke(width = if (isBlack) 0.6f else 1f)
            )
            if (Point(r, c) in overlay.deadStones) {
                val s = stoneR * 0.55f
                val x = Color(0xFFB23A2E)
                drawLine(x, Offset(center.x - s, center.y - s), Offset(center.x + s, center.y + s), strokeWidth = 3f)
                drawLine(x, Offset(center.x - s, center.y + s), Offset(center.x + s, center.y - s), strokeWidth = 3f)
            }
            // Move number overlay.
            overlay.moveNumbers[Point(r, c)]?.let { n ->
                val labelStyle = androidx.compose.ui.text.TextStyle(
                    color = if (isBlack) Color.White else Color.Black,
                    fontSize = (stoneR * 0.85f).coerceAtMost(18f).sp
                )
                val layout = measurer.measure(n.toString(), labelStyle)
                drawText(layout, topLeft = Offset(
                    center.x - layout.size.width / 2f,
                    center.y - layout.size.height / 2f
                ))
            }
        }

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

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawSoftShadow(topLeft: Offset, sz: Size) {
    val color = Color(0x1F000000)
    for (i in 0 until 3) {
        val grow = (i + 1) * 4f
        drawRect(
            color = color.copy(alpha = color.alpha / (i + 2f)),
            topLeft = Offset(topLeft.x - grow, topLeft.y - grow + 2f),
            size = Size(sz.width + grow * 2, sz.height + grow * 2)
        )
    }
}

private fun colLetter(col: Int): String {
    // Western Go convention: A-T excluding I. So col 0..18 maps to A,B,C,D,E,F,G,H,J,K,...,T.
    val skipI = 'I' - 'A'  // 8
    val ch = if (col < skipI) 'A' + col else 'A' + col + 1
    return ch.toString()
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

