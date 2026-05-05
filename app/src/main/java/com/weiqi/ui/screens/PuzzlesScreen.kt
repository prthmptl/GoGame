package com.weiqi.ui.screens

import androidx.compose.animation.AnimatedVisibility
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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.Lightbulb
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.weiqi.data.BoardTheme
import com.weiqi.data.SettingsStore
import com.weiqi.puzzle.Puzzle
import com.weiqi.puzzle.PuzzleEngine
import com.weiqi.puzzle.PuzzleLibrary
import com.weiqi.puzzle.PuzzleProgress
import com.weiqi.puzzle.PuzzleTier
import com.weiqi.ui.board.BoardAppearance
import com.weiqi.ui.board.BoardCanvas
import com.weiqi.ui.board.BoardOverlay
import com.weiqi.ui.components.ZenCard
import com.weiqi.ui.components.ZenChip

@Composable
fun PuzzlesScreen() {
    val ctx = LocalContext.current
    val progress = remember { PuzzleProgress.get(ctx) }
    val solved by progress.solved.collectAsState()
    var openedPuzzle by remember { mutableStateOf<Puzzle?>(null) }

    if (openedPuzzle != null) {
        PuzzleDetail(
            puzzle = openedPuzzle!!,
            onSolved = { progress.markSolved(openedPuzzle!!.id) },
            onBack = { openedPuzzle = null }
        )
    } else {
        PuzzleList(
            solvedIds = solved,
            onPick = { openedPuzzle = it }
        )
    }
}

@Composable
private fun PuzzleList(
    solvedIds: Set<String>,
    onPick: (Puzzle) -> Unit
) {
    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text("Tsumego",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text("Puzzles",
            style = MaterialTheme.typography.headlineMedium,
            fontWeight = FontWeight.SemiBold)
        Text(
            "Sharpen your tactical vision. Tap any problem to solve it.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        PuzzleTier.entries.forEach { tier ->
            val tierPuzzles = PuzzleLibrary.byTier[tier].orEmpty()
            val solvedCount = tierPuzzles.count { it.id in solvedIds }
            ZenCard(Modifier.fillMaxWidth()) {
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(tier.label,
                                style = MaterialTheme.typography.headlineSmall,
                                fontWeight = FontWeight.SemiBold)
                            Text(tier.description,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        ZenChip("$solvedCount/${tierPuzzles.size}",
                            container = MaterialTheme.colorScheme.surfaceContainerHigh)
                    }
                    tierPuzzles.forEach { puzzle ->
                        PuzzleRow(
                            puzzle = puzzle,
                            isSolved = puzzle.id in solvedIds,
                            onClick = { onPick(puzzle) }
                        )
                    }
                }
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

@Composable
private fun PuzzleRow(puzzle: Puzzle, isSolved: Boolean, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .clickable { onClick() }
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            Modifier
                .size(40.dp),
            contentAlignment = Alignment.Center
        ) {
            if (isSolved) {
                Icon(
                    Icons.Filled.CheckCircle,
                    contentDescription = "Solved",
                    tint = MaterialTheme.colorScheme.primary
                )
            } else {
                Box(
                    Modifier
                        .size(28.dp)
                        .background(
                            MaterialTheme.colorScheme.surfaceContainerHigh,
                            RoundedCornerShape(8.dp)
                        )
                )
            }
        }
        Spacer(Modifier.size(12.dp))
        Column(Modifier.weight(1f)) {
            Text(puzzle.title,
                style = MaterialTheme.typography.titleLarge.copy(fontSize = MaterialTheme.typography.bodyLarge.fontSize),
                fontWeight = FontWeight.SemiBold)
            Text("${puzzle.theme} · ${puzzle.toPlay.name} to play",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Icon(Icons.Filled.ChevronRight, contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun PuzzleDetail(
    puzzle: Puzzle,
    onSolved: () -> Unit,
    onBack: () -> Unit
) {
    val ctx = LocalContext.current
    val settings by SettingsStore.get(ctx).state.collectAsState()
    val appearance = remember(settings.boardTheme) {
        when (settings.boardTheme) {
            BoardTheme.CLASSIC_WOOD -> BoardAppearance.ClassicWood
            BoardTheme.MINIMAL_PAPER -> BoardAppearance.MinimalPaper
            BoardTheme.DARK_SLATE -> BoardAppearance.DarkSlate
            BoardTheme.HIGH_CONTRAST -> BoardAppearance.HighContrast
        }
    }

    var session by remember(puzzle.id) { mutableStateOf(PuzzleEngine.start(puzzle)) }

    LaunchedEffect(session.solved) {
        if (session.solved) onSolved()
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
            }
            Column(Modifier.weight(1f)) {
                Text(puzzle.tier.label,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(puzzle.title,
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold)
            }
            ZenChip(session.progressLabel)
        }

        ZenCard(
            Modifier.fillMaxWidth(),
            container = MaterialTheme.colorScheme.surfaceContainerLow
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(puzzle.description, style = MaterialTheme.typography.bodyMedium)
                AnimatedVisibility(visible = session.showHint) {
                    Text("Hint: ${puzzle.hint}",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.primary)
                }
            }
        }

        Box(Modifier.fillMaxWidth().aspectRatio(1f)) {
            BoardCanvas(
                board = session.state.board,
                overlay = BoardOverlay(
                    lastMove = session.state.lastMove?.point
                ),
                appearance = appearance,
                onTap = { p -> if (!session.solved) session = PuzzleEngine.userTap(session, p) }
            )
        }

        // Feedback line.
        Box(Modifier.fillMaxWidth().heightIn(min = 24.dp)) {
            session.message?.let { msg ->
                Text(
                    msg,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                    color = if (session.solved)
                        MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }

        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedButton(
                onClick = { session = PuzzleEngine.toggleHint(session) },
                modifier = Modifier.weight(1f).height(52.dp),
                shape = RoundedCornerShape(14.dp)
            ) {
                Icon(Icons.Filled.Lightbulb, contentDescription = null)
                Spacer(Modifier.size(6.dp))
                Text("HINT", style = MaterialTheme.typography.labelLarge)
            }
            OutlinedButton(
                onClick = { session = PuzzleEngine.reset(session) },
                modifier = Modifier.weight(1f).height(52.dp),
                shape = RoundedCornerShape(14.dp)
            ) {
                Icon(Icons.Filled.Refresh, contentDescription = null)
                Spacer(Modifier.size(6.dp))
                Text("RESET", style = MaterialTheme.typography.labelLarge)
            }
        }

        if (session.solved) {
            Button(
                onClick = onBack,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary,
                    contentColor = MaterialTheme.colorScheme.onPrimary
                ),
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth().height(56.dp)
            ) {
                Text("BACK TO LIST", style = MaterialTheme.typography.labelLarge)
            }
        }

        Text(
            "Mistakes: ${session.mistakes}",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
