package com.weiqi.ui.screens

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FileDownload
import androidx.compose.material.icons.filled.FileOpen
import androidx.compose.material.icons.filled.FirstPage
import androidx.compose.material.icons.automirrored.filled.LastPage
import androidx.compose.material.icons.automirrored.filled.NavigateBefore
import androidx.compose.material.icons.automirrored.filled.NavigateNext
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
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
import com.weiqi.data.SavedGameRepo
import com.weiqi.data.SettingsStore
import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.sgf.SgfImport
import com.weiqi.ui.board.BoardAppearance
import com.weiqi.ui.board.BoardCanvas
import com.weiqi.ui.board.BoardOverlay
import com.weiqi.ui.components.ZenCard
import java.io.File

@Composable
fun ReviewScreen(
    savedGameId: String? = null,
    repo: SavedGameRepo? = null
) {
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

    var loaded by remember { mutableStateOf<GameState?>(null) }
    var sgfText by remember { mutableStateOf<String?>(null) }
    var index by remember { mutableStateOf(0) }

    LaunchedEffect(savedGameId) {
        if (savedGameId != null && repo != null) {
            val entity = repo.get(savedGameId)
            val path = entity?.sgfPath
            if (!path.isNullOrBlank()) {
                runCatching {
                    val text = File(path).readText()
                    val state = SgfImport.import(text)
                    sgfText = text
                    loaded = state
                    index = state.history.size
                }
            }
        }
    }

    val pickSgf = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) {
            ctx.contentResolver.openInputStream(uri)?.use { stream ->
                val text = stream.bufferedReader().readText()
                sgfText = text
                loaded = SgfImport.import(text)
                index = (loaded?.history?.size ?: 0)
            }
        }
    }

    val saveSgf = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.CreateDocument("application/x-go-sgf")
    ) { uri ->
        val text = sgfText ?: return@rememberLauncherForActivityResult
        if (uri != null) {
            ctx.contentResolver.openOutputStream(uri)?.use { it.write(text.toByteArray()) }
        }
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        if (loaded == null) {
            EmptyReviewCard(
                onImport = {
                    pickSgf.launch(
                        arrayOf(
                            "application/x-go-sgf",
                            "application/octet-stream",
                            "text/plain",
                            "*/*"
                        )
                    )
                }
            )
            return@Column
        }
        val game = loaded!!
        val total = game.history.size
        val moveIndex = index.coerceIn(0, total)
        val replayed = remember(loaded, moveIndex) { replay(game.config, game, moveIndex) }
        val moveNumbers = remember(replayed, settings.showMoveNumbers) {
            if (!settings.showMoveNumbers) emptyMap()
            else buildMoveNumberMap(replayed)
        }

        ZenCard(
            Modifier.fillMaxWidth(),
            container = MaterialTheme.colorScheme.surfaceContainerLow
        ) {
            Row(
                Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Column(Modifier.weight(1f)) {
                    Text("Replay",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Move $moveIndex / $total",
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.SemiBold)
                }
                if (sgfText != null) {
                    IconButton(onClick = {
                        saveSgf.launch("game-${System.currentTimeMillis()}.sgf")
                    }) {
                        Icon(Icons.Filled.FileDownload, contentDescription = "Export SGF")
                    }
                }
                IconButton(onClick = { loaded = null; sgfText = null }) {
                    Icon(Icons.Filled.FileOpen, contentDescription = "Open another")
                }
            }
        }

        Box(Modifier.fillMaxWidth().aspectRatio(1f)) {
            BoardCanvas(
                board = replayed.board,
                overlay = BoardOverlay(
                    lastMove = replayed.lastMove?.point,
                    moveNumbers = moveNumbers
                ),
                appearance = appearance,
                onTap = { /* read-only */ }
            )
        }

        Slider(
            value = moveIndex.toFloat(),
            onValueChange = { index = it.toInt() },
            valueRange = 0f..total.toFloat().coerceAtLeast(1f),
            steps = (total - 1).coerceAtLeast(0)
        )

        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceEvenly
        ) {
            IconButton(onClick = { index = 0 }) {
                Icon(Icons.Filled.FirstPage, contentDescription = "Start")
            }
            IconButton(onClick = { if (index > 0) index-- }) {
                Icon(Icons.AutoMirrored.Filled.NavigateBefore, contentDescription = "Previous")
            }
            IconButton(onClick = { if (index < total) index++ }) {
                Icon(Icons.AutoMirrored.Filled.NavigateNext, contentDescription = "Next")
            }
            IconButton(onClick = { index = total }) {
                Icon(Icons.AutoMirrored.Filled.LastPage, contentDescription = "End")
            }
        }
    }
}

@Composable
private fun EmptyReviewCard(onImport: () -> Unit) {
    ZenCard(
        modifier = Modifier.fillMaxWidth(),
        container = MaterialTheme.colorScheme.surfaceContainerLow
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("Review a game",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.SemiBold)
            Text(
                "Import an SGF file to step through it move by move. " +
                    "Finished games are also saved here automatically — tap one on the home screen to open it.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Button(
                onClick = onImport,
                shape = RoundedCornerShape(14.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary,
                    contentColor = MaterialTheme.colorScheme.onPrimary
                ),
                modifier = Modifier.fillMaxWidth().height(56.dp)
            ) {
                Icon(Icons.Filled.FileOpen, contentDescription = null)
                Spacer(Modifier.padding(start = 8.dp))
                Text("IMPORT SGF", style = MaterialTheme.typography.labelLarge)
            }
        }
    }
}

private fun replay(config: GameConfig, source: GameState, upTo: Int): GameState {
    var s = GameState.newGame(config)
    val moves = source.history.take(upTo)
    for (m in moves) {
        val intent = when (m.type) {
            MoveType.PASS -> MoveIntent(MoveType.PASS)
            MoveType.RESIGN -> MoveIntent(MoveType.RESIGN)
            MoveType.PLACE_STONE -> MoveIntent(MoveType.PLACE_STONE, m.point)
        }
        val r = Rules.apply(s, intent)
        if (r is MoveResult.Accepted) s = r.newState else break
    }
    return s
}

private fun buildMoveNumberMap(state: GameState): Map<Point, Int> {
    val out = HashMap<Point, Int>()
    state.history.forEach { m ->
        if (m.type == MoveType.PLACE_STONE && m.point != null && state.board.get(m.point) != com.weiqi.engine.CellState.EMPTY) {
            out[m.point] = m.moveNumber
        }
    }
    return out
}
