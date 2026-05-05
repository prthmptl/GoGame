package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.weiqi.ai.AiDifficulty
import com.weiqi.engine.GameConfig
import com.weiqi.engine.StoneColor
import com.weiqi.ui.components.ZenCard

data class GameSetup(
    val config: GameConfig,
    val opponent: Opponent,
    val aiColor: StoneColor,
    val aiDifficulty: AiDifficulty
)

@Composable
fun SetupScreen(
    isAi: Boolean,
    onStart: (GameSetup) -> Unit
) {
    var size by remember { mutableIntStateOf(9) }
    var komi by remember { mutableStateOf(7.5) }
    var handicap by remember { mutableIntStateOf(0) }
    var aiColor by remember { mutableStateOf(StoneColor.WHITE) }
    var aiDifficulty by remember { mutableStateOf(AiDifficulty.BEGINNER) }

    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Column {
            Text(if (isAi) "Vs. AI" else "Local Match",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Game setup",
                style = MaterialTheme.typography.headlineMedium,
                fontWeight = FontWeight.SemiBold)
        }

        ZenCard(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.spacedBy(20.dp)
            ) {
                ChipSection(
                    title = "BOARD SIZE",
                    options = listOf(9, 13, 19),
                    isSelected = { it == size },
                    label = { "${it}×${it}" },
                    onSelect = { size = it }
                )
                ChipSection(
                    title = "KOMI",
                    options = listOf(0.5, 5.5, 6.5, 7.5),
                    isSelected = { it == komi },
                    label = { it.toString() },
                    onSelect = { komi = it }
                )
                Column {
                    SectionLabel("HANDICAP")
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        listOf(
                            listOf(0, 2, 3),
                            listOf(4, 5, 6),
                            listOf(7, 8, 9)
                        ).forEach { rowOpts ->
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                rowOpts.forEach { h ->
                                    FilterChip(
                                        selected = handicap == h,
                                        onClick = { handicap = h },
                                        label = { Text("$h") },
                                        shape = RoundedCornerShape(14.dp),
                                        modifier = Modifier.weight(1f)
                                    )
                                }
                            }
                        }
                    }
                }
                if (isAi) {
                    Column {
                        SectionLabel("AI PLAYS")
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            listOf(StoneColor.BLACK, StoneColor.WHITE).forEach { c ->
                                FilterChip(
                                    selected = aiColor == c,
                                    onClick = { aiColor = c },
                                    label = { Text(c.name) },
                                    shape = RoundedCornerShape(14.dp)
                                )
                            }
                        }
                    }
                    Column {
                        SectionLabel("AI DIFFICULTY")
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            AiDifficulty.entries.forEach { d ->
                                FilterChip(
                                    selected = aiDifficulty == d,
                                    onClick = { aiDifficulty = d },
                                    label = { Text(d.label) },
                                    shape = RoundedCornerShape(14.dp)
                                )
                            }
                        }
                        Text(
                            aiDifficulty.description,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 6.dp)
                        )
                    }
                }
            }
        }

        Button(
            onClick = {
                onStart(
                    GameSetup(
                        config = GameConfig(boardSize = size, komi = komi, handicap = handicap),
                        opponent = if (isAi) Opponent.AI else Opponent.HUMAN,
                        aiColor = aiColor,
                        aiDifficulty = aiDifficulty
                    )
                )
            },
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary
            ),
            shape = RoundedCornerShape(16.dp),
            modifier = Modifier
                .fillMaxWidth()
                .height(60.dp)
        ) {
            Text("BEGIN GAME", style = MaterialTheme.typography.labelLarge)
        }
    }
}

@Composable
private fun <T> ChipSection(
    title: String,
    options: List<T>,
    isSelected: (T) -> Boolean,
    label: (T) -> String,
    onSelect: (T) -> Unit
) {
    Column {
        SectionLabel(title)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            options.forEach { opt ->
                FilterChip(
                    selected = isSelected(opt),
                    onClick = { onSelect(opt) },
                    label = { Text(label(opt)) },
                    shape = RoundedCornerShape(14.dp)
                )
            }
        }
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.padding(bottom = 8.dp)
    )
}
