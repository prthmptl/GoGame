package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.unit.dp
import com.weiqi.engine.Board
import com.weiqi.engine.CellState
import com.weiqi.engine.Point
import com.weiqi.ui.board.BoardAppearance
import com.weiqi.ui.board.BoardCanvas
import com.weiqi.ui.board.BoardOverlay
import com.weiqi.ui.components.ZenCard

private data class Diagram(
    val boardSize: Int,
    val black: List<Point> = emptyList(),
    val white: List<Point> = emptyList(),
    val markers: Set<Point> = emptySet(),
    val caption: String
)

private data class Lesson(
    val title: String,
    val summary: String,
    val body: String,
    val diagrams: List<Diagram>
)

private val lessons = listOf(
    Lesson(
        title = "Liberties",
        summary = "Every stone has empty neighbors keeping it alive.",
        body = "A stone's liberties are the empty intersections directly adjacent to it (up, down, left, right). " +
            "Connected stones of the same color share liberties. " +
            "When a group has zero liberties, it is captured and removed from the board. " +
            "Edges and corners reduce the number of liberties — a corner stone starts with only two.",
        diagrams = listOf(
            Diagram(
                boardSize = 7,
                black = listOf(Point(3, 3)),
                markers = setOf(Point(2, 3), Point(4, 3), Point(3, 2), Point(3, 4)),
                caption = "Center stone has 4 liberties (red dots)."
            ),
            Diagram(
                boardSize = 7,
                black = listOf(Point(0, 0)),
                markers = setOf(Point(1, 0), Point(0, 1)),
                caption = "Corner stone has only 2 liberties."
            )
        )
    ),
    Lesson(
        title = "Capturing",
        summary = "Surround your opponent to remove their stones.",
        body = "To capture, you must take away the last liberty of an opposing group. " +
            "If your move would self-capture (suicide), it is illegal — unless the same move captures " +
            "an opposing group first, restoring liberties to your own. " +
            "Captured stones are returned to the bowl and the points become empty again.",
        diagrams = listOf(
            Diagram(
                boardSize = 7,
                white = listOf(Point(3, 3)),
                black = listOf(Point(2, 3), Point(4, 3), Point(3, 2)),
                markers = setOf(Point(3, 4)),
                caption = "Black plays at the marked point — White is captured."
            )
        )
    ),
    Lesson(
        title = "Eyes and Life",
        summary = "Two true eyes make a group permanently alive.",
        body = "An eye is an empty point inside a group that the opponent cannot fill. " +
            "A group with two separate eyes can never be captured: filling either eye would be suicide. " +
            "Strong shapes form two eyes early; weak shapes are reduced to one eye and die. " +
            "Beware false eyes — points that look like eyes but can be taken away tactically.",
        diagrams = listOf(
            Diagram(
                boardSize = 7,
                black = listOf(
                    Point(0, 1), Point(0, 2), Point(0, 3), Point(0, 4),
                    Point(1, 0), Point(1, 4),
                    Point(2, 0), Point(2, 4),
                    Point(3, 0), Point(3, 1), Point(3, 2), Point(3, 3), Point(3, 4)
                ),
                markers = setOf(Point(1, 2), Point(2, 2)),
                caption = "Two eyes (red marks). White cannot fill either — alive forever."
            )
        )
    ),
    Lesson(
        title = "Ko",
        summary = "You cannot immediately recreate the previous position.",
        body = "After a one-stone capture, your opponent cannot recapture if doing so would return the board " +
            "to the position just before their previous move. They must play elsewhere first (a ko threat). " +
            "Positional superko extends this rule: no move may recreate any earlier whole-board position. " +
            "Ko fights are about who has more meaningful threats elsewhere on the board.",
        diagrams = listOf(
            Diagram(
                boardSize = 7,
                black = listOf(Point(2, 3), Point(3, 4), Point(4, 3)),
                white = listOf(Point(2, 2), Point(3, 1), Point(4, 2), Point(3, 3)),
                markers = setOf(Point(3, 2)),
                caption = "Marked point: a classic ko shape between Black and White."
            )
        )
    ),
    Lesson(
        title = "Scoring (Chinese)",
        summary = "Stones on the board + surrounded territory + komi.",
        body = "After both players pass, the game enters scoring. Mark dead stones (groups that cannot make two eyes) " +
            "for removal. Then count: each player's score is their stones still on the board, plus the empty " +
            "points surrounded only by their color. White also receives komi (typically 7.5) to compensate " +
            "Black's first-move advantage. Highest total wins.",
        diagrams = listOf(
            Diagram(
                boardSize = 7,
                black = listOf(Point(0, 3), Point(1, 3), Point(2, 3), Point(3, 3),
                    Point(3, 2), Point(3, 1), Point(3, 0)),
                white = listOf(Point(0, 4), Point(1, 4), Point(2, 4), Point(3, 4),
                    Point(4, 3), Point(4, 4), Point(4, 5), Point(4, 6)),
                markers = emptySet(),
                caption = "Black surrounds the top-left; White surrounds the rest. Each empty region counts for the side that surrounds it."
            )
        )
    )
)

@Composable
fun TutorialScreen() {
    var current by remember { mutableStateOf<Lesson?>(null) }
    if (current == null) {
        TutorialList(onPick = { current = it })
    } else {
        TutorialDetail(lesson = current!!, onBack = { current = null })
    }
}

@Composable
private fun TutorialList(onPick: (Lesson) -> Unit) {
    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text("Learn",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text("Beginner Path",
            style = MaterialTheme.typography.headlineMedium,
            fontWeight = FontWeight.SemiBold)
        Text(
            "Five short lessons covering everything you need to play your first full game.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        lessons.forEachIndexed { i, lesson ->
            ZenCard(
                modifier = Modifier.fillMaxWidth().clickable { onPick(lesson) }
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Lesson ${i + 1}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(lesson.title,
                            style = MaterialTheme.typography.headlineSmall)
                        Text(lesson.summary,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Icon(Icons.Filled.ChevronRight, contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}

@Composable
private fun TutorialDetail(lesson: Lesson, onBack: () -> Unit) {
    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
            }
            Text("Beginner path",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(lesson.title,
            style = MaterialTheme.typography.displayLarge,
            color = MaterialTheme.colorScheme.onSurface)
        Text(lesson.summary,
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(4.dp))
        ZenCard(Modifier.fillMaxWidth()) {
            Text(lesson.body,
                style = MaterialTheme.typography.bodyMedium)
        }
        lesson.diagrams.forEach { diagram ->
            DiagramCard(diagram)
        }
        TextButton(onClick = onBack, modifier = Modifier.fillMaxWidth()) {
            Text("Back to lessons")
        }
    }
}

@Composable
private fun DiagramCard(diagram: Diagram) {
    val board = remember(diagram) {
        var b = Board.empty(diagram.boardSize)
        for (p in diagram.black) b = b.set(p, CellState.BLACK)
        for (p in diagram.white) b = b.set(p, CellState.WHITE)
        b
    }
    ZenCard(
        modifier = Modifier.fillMaxWidth(),
        container = MaterialTheme.colorScheme.surfaceContainerLow
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Box(
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(1f)
            ) {
                BoardCanvas(
                    board = board,
                    overlay = BoardOverlay(markers = diagram.markers),
                    appearance = BoardAppearance.ClassicWood,
                    onTap = { /* read-only */ }
                )
            }
            Text(diagram.caption,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
