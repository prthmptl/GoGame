package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.weiqi.engine.GameConfig
import com.weiqi.engine.StoneColor
import com.weiqi.ui.components.ZenCard

@Composable
fun SetupScreen(
    isAi: Boolean,
    onStart: (GameConfig, Opponent, StoneColor) -> Unit
) {
    var size by remember { mutableIntStateOf(9) }
    var komi by remember { mutableStateOf(7.5) }
    var handicap by remember { mutableIntStateOf(0) }
    var aiColor by remember { mutableStateOf(StoneColor.WHITE) }

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
                ChipSection("BOARD SIZE", listOf(9, 13, 19), size, { it == size }) { size = it; "${it}×${it}" }
                ChipSection("KOMI", listOf(0.5, 5.5, 6.5, 7.5), komi, { it == komi }) { komi = it; it.toString() }
                ChipSection("HANDICAP", listOf(0, 2, 3, 4, 5, 6, 7, 8, 9), handicap, { it == handicap }) { handicap = it; "$it" }
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
                }
            }
        }

        Button(
            onClick = {
                val cfg = GameConfig(boardSize = size, komi = komi, handicap = handicap)
                val opp = if (isAi) Opponent.AI else Opponent.HUMAN
                onStart(cfg, opp, aiColor)
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
    @Suppress("UNUSED_PARAMETER") current: T,
    isSelected: (T) -> Boolean,
    onSelect: (T) -> String
) {
    Column {
        SectionLabel(title)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            options.forEach { opt ->
                val label = remember(opt) { opt.toString() }
                FilterChip(
                    selected = isSelected(opt),
                    onClick = { onSelect(opt) },
                    label = { Text(label) },
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
