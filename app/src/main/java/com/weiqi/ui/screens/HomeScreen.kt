package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.MenuBook
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.IosShare
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.weiqi.engine.StoneColor
import com.weiqi.ui.board.MiniStone
import com.weiqi.ui.components.ZenCard

data class RecentGame(
    val id: String,
    val opponent: String,
    val result: String,
    val boardSize: Int,
    val date: String,
    val youPlayed: StoneColor,
    val sgfPath: String?
)

@Composable
fun HomeScreen(
    onPlayLocal: () -> Unit,
    onPlayAi: () -> Unit,
    onRules: () -> Unit,
    onResume: (() -> Unit)? = null,
    recents: List<RecentGame> = emptyList(),
    onOpenRecent: (RecentGame) -> Unit = {},
    onShareRecent: (RecentGame) -> Unit = {},
    onDeleteRecent: (RecentGame) -> Unit = {}
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        // Hero.
        ZenCard(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 260.dp),
            container = MaterialTheme.colorScheme.surfaceContainerLow
        ) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(12.dp),
                modifier = Modifier.fillMaxSize()
            ) {
                Text(
                    "Find Your Focus",
                    style = MaterialTheme.typography.displayLarge.copy(lineHeight = 48.sp),
                    color = MaterialTheme.colorScheme.onSurface,
                    textAlign = TextAlign.Center
                )
                Text(
                    "Step into the quiet space.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    textAlign = TextAlign.Center
                )
                Spacer(Modifier.weight(1f, fill = false))
                Button(
                    onClick = onPlayLocal,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = MaterialTheme.colorScheme.primary,
                        contentColor = MaterialTheme.colorScheme.onPrimary
                    ),
                    shape = RoundedCornerShape(16.dp),
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(56.dp)
                ) {
                    Icon(Icons.Filled.PlayArrow, contentDescription = null)
                    Spacer(Modifier.size(6.dp))
                    Text("Play Now", style = MaterialTheme.typography.labelLarge)
                }
            }
        }

        // Rules reference.
        ZenCard(
            modifier = Modifier.fillMaxWidth().clickable { onRules() },
            container = MaterialTheme.colorScheme.surfaceContainerHigh
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(Modifier.weight(1f)) {
                    Text("Rules", style = MaterialTheme.typography.headlineSmall)
                    Text("Complete Chinese rules reference.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Icon(
                    Icons.AutoMirrored.Filled.MenuBook,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary
                )
            }
        }

        if (onResume != null) {
            ZenCard(
                modifier = Modifier.fillMaxWidth().clickable { onResume() },
                container = MaterialTheme.colorScheme.primaryContainer
            ) {
                Row(
                    Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(Modifier.weight(1f)) {
                        Text("Continue Game",
                            style = MaterialTheme.typography.headlineSmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer)
                        Text("Pick up where you left off.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.85f))
                    }
                    MiniStone(StoneColor.WHITE, size = 36.dp)
                }
            }
        }

        // Play vs AI.
        ZenCard(
            modifier = Modifier.fillMaxWidth().clickable { onPlayAi() },
            container = MaterialTheme.colorScheme.surfaceContainerHigh
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(Modifier.weight(1f)) {
                    Text("Play vs AI", style = MaterialTheme.typography.headlineSmall)
                    Text("Beginner or Intermediate engine.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                MiniStone(StoneColor.BLACK, size = 36.dp)
            }
        }

        // Recent games — only render the card if there is anything to show.
        if (recents.isNotEmpty()) {
            ZenCard(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.fillMaxSize()) {
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("Recent Games", style = MaterialTheme.typography.headlineSmall)
                        Text("LATEST ${recents.size}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.primary)
                    }
                    Spacer(Modifier.height(8.dp))
                    recents.forEach { game ->
                        RecentGameRow(
                            game,
                            onOpen = { onOpenRecent(game) },
                            onShare = { onShareRecent(game) },
                            onDelete = { onDeleteRecent(game) }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun RecentGameRow(
    game: RecentGame,
    onOpen: () -> Unit,
    onShare: () -> Unit,
    onDelete: () -> Unit
) {
    var showDeleteConfirm by remember { mutableStateOf(false) }

    if (showDeleteConfirm) {
        AlertDialog(
            onDismissRequest = { showDeleteConfirm = false },
            title = { Text("Delete recent game?") },
            text = { Text("This removes the saved game and its SGF file from your history.") },
            confirmButton = {
                TextButton(
                    onClick = {
                        showDeleteConfirm = false
                        onDelete()
                    }
                ) {
                    Text("Delete")
                }
            },
            dismissButton = {
                TextButton(onClick = { showDeleteConfirm = false }) {
                    Text("Cancel")
                }
            }
        )
    }

    Row(
        Modifier
            .fillMaxWidth()
            .clickable(enabled = game.sgfPath != null) { onOpen() }
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            Modifier
                .size(36.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.surfaceContainerHigh),
            contentAlignment = Alignment.Center
        ) { MiniStone(game.youPlayed, size = 24.dp) }
        Spacer(Modifier.size(12.dp))
        Column(Modifier.weight(1f)) {
            Text("vs. ${game.opponent}",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold)
            Text("${game.result} · ${game.boardSize}×${game.boardSize} · ${game.date}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        if (game.sgfPath != null) {
            IconButton(onClick = onShare) {
                Icon(
                    Icons.Filled.IosShare,
                    contentDescription = "Share SGF",
                    tint = MaterialTheme.colorScheme.primary
                )
            }
        }
        IconButton(onClick = { showDeleteConfirm = true }) {
            Icon(
                Icons.Filled.Delete,
                contentDescription = "Delete recent game",
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}
