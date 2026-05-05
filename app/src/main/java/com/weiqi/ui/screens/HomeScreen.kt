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
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import com.weiqi.ui.components.ZenChip
import com.weiqi.ui.theme.Zen

data class RecentGame(
    val opponent: String,
    val result: String,
    val boardSize: Int,
    val date: String,
    val youPlayed: StoneColor
)

@Composable
fun HomeScreen(
    onPlayLocal: () -> Unit,
    onPlayAi: () -> Unit,
    onPuzzles: () -> Unit,
    onRules: () -> Unit,
    onResume: (() -> Unit)? = null,
    recents: List<RecentGame> = emptyList()
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

        // Daily puzzle.
        ZenCard(
            modifier = Modifier.fillMaxWidth().heightIn(min = 200.dp).clickable { onPuzzles() }
        ) {
            Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Row(
                    Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("Puzzles", style = MaterialTheme.typography.headlineSmall)
                    ZenChip("LIFE & DEATH")
                }
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(min = 120.dp)
                        .clip(RoundedCornerShape(12.dp))
                        .background(Zen.kayaWood),
                    contentAlignment = Alignment.Center
                ) {
                    Row(horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                        MiniStone(StoneColor.BLACK, size = 26.dp)
                        MiniStone(StoneColor.WHITE, size = 26.dp)
                        MiniStone(StoneColor.BLACK, size = 26.dp)
                    }
                }
                Row(
                    Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("Sharpen your reading.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("TAP TO SOLVE", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary)
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
                        RecentGameRow(game)
                    }
                }
            }
        }
    }
}

@Composable
private fun RecentGameRow(game: RecentGame) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 8.dp),
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
            Text("${game.result} · ${game.boardSize}×${game.boardSize}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(game.date,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
