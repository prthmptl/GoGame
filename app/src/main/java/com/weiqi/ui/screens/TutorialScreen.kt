package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
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
import com.weiqi.ui.components.ZenCard

private data class Lesson(val title: String, val summary: String, val body: String)

private val lessons = listOf(
    Lesson(
        "Liberties",
        "Every stone has empty neighbors keeping it alive.",
        "A stone's liberties are the empty intersections directly adjacent to it (up, down, left, right). " +
            "Connected stones of the same color share liberties. " +
            "When a group has zero liberties, it is captured and removed from the board. " +
            "Edges and corners reduce the number of liberties — a corner stone starts with only two."
    ),
    Lesson(
        "Capturing",
        "Surround your opponent to remove their stones.",
        "To capture, you must take away the last liberty of an opposing group. " +
            "If your move would self-capture (suicide), it is illegal — unless the same move captures " +
            "an opposing group first, restoring liberties to your own. " +
            "Captured stones are returned to the bowl and the points become empty again."
    ),
    Lesson(
        "Eyes and Life",
        "Two true eyes make a group permanently alive.",
        "An eye is an empty point inside a group that the opponent cannot fill. " +
            "A group with two separate eyes can never be captured: filling either eye would be suicide. " +
            "Strong shapes form two eyes early; weak shapes are reduced to one eye and die. " +
            "Beware false eyes — points that look like eyes but can be taken away tactically."
    ),
    Lesson(
        "Ko",
        "You cannot immediately recreate the previous position.",
        "After a one-stone capture, your opponent cannot recapture if doing so would return the board " +
            "to the position just before their previous move. They must play elsewhere first (a ko threat). " +
            "Positional superko extends this rule: no move may recreate any earlier whole-board position. " +
            "Ko fights are about who has more meaningful threats elsewhere on the board."
    ),
    Lesson(
        "Scoring (Chinese)",
        "Stones on the board + surrounded territory + komi.",
        "After both players pass, the game enters scoring. Mark dead stones (groups that cannot make two eyes) " +
            "for removal. Then count: each player's score is their stones still on the board, plus the empty " +
            "points surrounded only by their color. White also receives komi (typically 7.5) to compensate " +
            "Black's first-move advantage. Highest total wins."
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
                Icon(Icons.Filled.ArrowBack, contentDescription = "Back")
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
        TextButton(onClick = onBack, modifier = Modifier.fillMaxWidth()) {
            Text("Back to lessons")
        }
    }
}
