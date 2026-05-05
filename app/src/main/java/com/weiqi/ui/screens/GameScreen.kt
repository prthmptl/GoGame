package com.weiqi.ui.screens

import android.content.Intent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
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
import androidx.lifecycle.viewmodel.compose.viewModel
import com.weiqi.engine.GameStatus
import com.weiqi.engine.StoneColor
import com.weiqi.data.BoardTheme
import com.weiqi.data.SettingsStore
import com.weiqi.ui.board.BoardAppearance
import com.weiqi.ui.board.BoardCanvas
import com.weiqi.ui.board.BoardOverlay
import com.weiqi.ui.board.MiniStone
import com.weiqi.ui.components.ZenCard
import com.weiqi.ui.components.ZenChip

@Composable
fun GameScreen(
    vm: GameViewModel = viewModel(),
    onExit: () -> Unit
) {
    val ui by vm.ui.collectAsState()
    val state = ui.state
    val ctx = LocalContext.current
    val settings by SettingsStore.get(ctx).state.collectAsState()
    val appearance = remember(settings.boardTheme, settings.showCoordinates) {
        when (settings.boardTheme) {
            BoardTheme.CLASSIC_WOOD -> BoardAppearance.ClassicWood
            BoardTheme.MINIMAL_PAPER -> BoardAppearance.MinimalPaper
            BoardTheme.DARK_SLATE -> BoardAppearance.DarkSlate
            BoardTheme.HIGH_CONTRAST -> BoardAppearance.HighContrast
        }.copy(showCoordinates = settings.showCoordinates)
    }

    var showResignDialog by remember { mutableStateOf(false) }

    BoxWithConstraints(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        val boardSidePx = minOf(maxWidth.value, maxHeight.value * 0.58f)
        val boardSide = boardSidePx.dp

        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                ZenChip(
                    text = when (state.status) {
                        GameStatus.ACTIVE -> if (ui.opponent == Opponent.AI) "VS AI" else "LOCAL MATCH"
                        GameStatus.SCORING -> "SCORING"
                        GameStatus.COMPLETED -> if (ui.timeoutLoser != null) "TIMEOUT" else "COMPLETED"
                        GameStatus.RESIGNED -> "RESIGNED"
                    },
                    container = MaterialTheme.colorScheme.primary
                )
            }

            val oppColor = if (ui.opponent == Opponent.AI) ui.aiPlays else StoneColor.WHITE
            PlayerCard(
                color = oppColor,
                name = if (ui.opponent == Opponent.AI) "AI" else "Opponent",
                time = GameViewModel.formatTime(
                    if (oppColor == StoneColor.BLACK) ui.blackMillis else ui.whiteMillis
                ),
                active = state.status == GameStatus.ACTIVE && state.currentPlayer == oppColor,
                lowTime = lowTime(ui, oppColor)
            )

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(boardSide),
                contentAlignment = Alignment.Center
            ) {
                Box(Modifier.size(boardSide)) {
                    BoardCanvas(
                        board = state.board,
                        overlay = BoardOverlay(
                            lastMove = state.lastMove?.point,
                            koPoint = state.koPoint,
                            deadStones = ui.deadStones,
                            pending = ui.pendingPoint?.let { it to state.currentPlayer }
                        ),
                        appearance = appearance,
                        onTap = { vm.tap(it) }
                    )
                }
            }

            val you = humanColor(ui)
            PlayerCard(
                color = you,
                name = "You",
                time = GameViewModel.formatTime(
                    if (you == StoneColor.BLACK) ui.blackMillis else ui.whiteMillis
                ),
                active = state.status == GameStatus.ACTIVE && state.currentPlayer == you,
                lowTime = lowTime(ui, you)
            )

            ui.rejection?.let {
                Text("⚠ $it",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.error)
            }
            if (ui.aiThinking) {
                Text("AI is thinking…",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (ui.pendingPoint != null && state.status == GameStatus.ACTIVE) {
                Text("Tap again to confirm — or tap elsewhere to choose a different point.",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary)
            }

            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.BottomCenter) {
                when (state.status) {
                    GameStatus.ACTIVE -> ActiveControls(
                        onPass = vm::pass,
                        onResign = { showResignDialog = true },
                        onUndo = vm::undo,
                        onExit = onExit
                    )
                    GameStatus.SCORING -> ScoringControls(
                        score = ui.score,
                        onConfirm = vm::confirmScore,
                        onResume = vm::resumePlay
                    )
                    GameStatus.COMPLETED, GameStatus.RESIGNED -> {
                        CompletedControls(
                            state = state,
                            score = ui.score,
                            timedOut = ui.timeoutLoser,
                            onExportSgf = {
                                val sgf = vm.exportSgf()
                                val send = Intent(Intent.ACTION_SEND).apply {
                                    type = "application/x-go-sgf"
                                    putExtra(Intent.EXTRA_TEXT, sgf)
                                    putExtra(Intent.EXTRA_SUBJECT, "Weiqi game (SGF)")
                                }
                                ctx.startActivity(Intent.createChooser(send, "Share SGF"))
                            },
                            onExit = onExit
                        )
                    }
                }
            }
        }
    }

    if (showResignDialog) {
        AlertDialog(
            onDismissRequest = { showResignDialog = false },
            title = { Text("Resign?") },
            text = { Text("This will end the game.") },
            confirmButton = {
                TextButton(onClick = { showResignDialog = false; vm.resign() }) { Text("Resign") }
            },
            dismissButton = {
                TextButton(onClick = { showResignDialog = false }) { Text("Cancel") }
            }
        )
    }
}

private fun lowTime(ui: GameUi, color: StoneColor): Boolean {
    val ms = if (color == StoneColor.BLACK) ui.blackMillis else ui.whiteMillis
    return ui.state.status == GameStatus.ACTIVE && ms in 1L..30_000L
}

@Composable
private fun PlayerCard(
    color: StoneColor,
    name: String,
    time: String,
    active: Boolean,
    lowTime: Boolean
) {
    ZenCard(
        modifier = Modifier.fillMaxWidth(),
        container = if (active)
            MaterialTheme.colorScheme.surfaceContainerHigh
        else
            MaterialTheme.colorScheme.surfaceContainerLow
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            MiniStone(color, size = 36.dp)
            Spacer(Modifier.size(12.dp))
            Column(Modifier.weight(1f)) {
                Text(name,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold)
                Text(if (active) "Thinking…" else "Waiting",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Text(
                time,
                style = MaterialTheme.typography.headlineSmall.copy(fontWeight = FontWeight.Bold),
                color = when {
                    lowTime -> MaterialTheme.colorScheme.error
                    active -> MaterialTheme.colorScheme.primary
                    else -> MaterialTheme.colorScheme.onSurfaceVariant
                }
            )
        }
    }
}

@Composable
private fun ActiveControls(
    onPass: () -> Unit,
    onResign: () -> Unit,
    onUndo: () -> Unit,
    onExit: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedButton(
                onClick = onPass,
                modifier = Modifier.weight(1f).heightIn(min = 52.dp),
                shape = RoundedCornerShape(14.dp)
            ) { Text("PASS", style = MaterialTheme.typography.labelLarge) }
            OutlinedButton(
                onClick = onUndo,
                modifier = Modifier.weight(1f).heightIn(min = 52.dp),
                shape = RoundedCornerShape(14.dp)
            ) { Text("UNDO", style = MaterialTheme.typography.labelLarge) }
        }
        Button(
            onClick = onResign,
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary
            ),
            shape = RoundedCornerShape(14.dp),
            modifier = Modifier.fillMaxWidth().heightIn(min = 56.dp)
        ) { Text("RESIGN", style = MaterialTheme.typography.labelLarge) }
        TextButton(
            onClick = onExit,
            modifier = Modifier.fillMaxWidth()
        ) { Text("Exit to menu") }
    }
}

@Composable
private fun ScoringControls(
    score: com.weiqi.engine.ScoreResult?,
    onConfirm: () -> Unit,
    onResume: () -> Unit
) {
    ZenCard(Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Tap stones to mark them dead",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold)
            score?.let { s ->
                Row(Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween) {
                    Text("Black area  ${s.blackArea}",
                        style = MaterialTheme.typography.bodyMedium)
                    Text("White total  %.1f".format(s.whiteTotal),
                        style = MaterialTheme.typography.bodyMedium)
                }
                Text(s.resultString,
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.primary)
            }
            Spacer(Modifier.size(4.dp))
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                OutlinedButton(
                    onClick = onResume,
                    modifier = Modifier.weight(1f).heightIn(min = 56.dp),
                    shape = RoundedCornerShape(14.dp)
                ) { Text("RESUME", style = MaterialTheme.typography.labelLarge) }
                Button(
                    onClick = onConfirm,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = MaterialTheme.colorScheme.primary,
                        contentColor = MaterialTheme.colorScheme.onPrimary
                    ),
                    modifier = Modifier.weight(1f).heightIn(min = 56.dp),
                    shape = RoundedCornerShape(14.dp)
                ) { Text("CONFIRM", style = MaterialTheme.typography.labelLarge) }
            }
        }
    }
}

@Composable
private fun CompletedControls(
    state: com.weiqi.engine.GameState,
    score: com.weiqi.engine.ScoreResult?,
    timedOut: StoneColor?,
    onExportSgf: () -> Unit,
    onExit: () -> Unit
) {
    ZenCard(Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            when {
                timedOut != null -> Text("${timedOut.other()} wins on time",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.primary)
                state.status == GameStatus.RESIGNED -> {
                    val winner = state.history.last().player.other()
                    Text("$winner wins by resignation",
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary)
                }
                else -> score?.let {
                    Text(it.resultString,
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary)
                    Spacer(Modifier.size(4.dp))
                    Text("Black",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Stones ${it.blackStones}  ·  Territory ${it.blackTerritory}  =  ${it.blackArea}",
                        style = MaterialTheme.typography.bodyMedium)
                    Spacer(Modifier.size(4.dp))
                    Text("White",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Stones ${it.whiteStones}  ·  Territory ${it.whiteTerritory}  ·  Komi %.1f  =  %.1f"
                        .format(it.komi, it.whiteTotal),
                        style = MaterialTheme.typography.bodyMedium)
                }
            }
            Text(
                "SGF (Smart Game Format) is the standard text format for sharing a game record.",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Button(
                onClick = onExportSgf,
                modifier = Modifier.fillMaxWidth().heightIn(min = 56.dp),
                shape = RoundedCornerShape(14.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary,
                    contentColor = MaterialTheme.colorScheme.onPrimary
                )
            ) { Text("SHARE SGF", style = MaterialTheme.typography.labelLarge) }
            TextButton(onClick = onExit, modifier = Modifier.fillMaxWidth()) {
                Text("Back to menu")
            }
        }
    }
}

private fun humanColor(ui: GameUi): StoneColor =
    if (ui.opponent == Opponent.AI) ui.aiPlays.other() else StoneColor.BLACK
