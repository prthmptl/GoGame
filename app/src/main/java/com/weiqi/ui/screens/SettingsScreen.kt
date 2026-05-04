package com.weiqi.ui.screens

import androidx.compose.foundation.background
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
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.weiqi.data.BoardTheme
import com.weiqi.data.SettingsStore
import com.weiqi.ui.components.ZenCard

@Composable
fun SettingsScreen() {
    val ctx = LocalContext.current
    val store = SettingsStore.get(ctx)
    val s by store.state.collectAsState()

    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text("Settings",
            style = MaterialTheme.typography.headlineMedium,
            fontWeight = FontWeight.SemiBold)

        ZenCard(Modifier.fillMaxWidth()) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("BOARD THEME",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                val opts = listOf(
                    BoardTheme.CLASSIC_WOOD to "Wood",
                    BoardTheme.MINIMAL_PAPER to "Paper",
                    BoardTheme.DARK_SLATE to "Slate",
                    BoardTheme.HIGH_CONTRAST to "High Contrast"
                )
                opts.chunked(2).forEach { rowOpts ->
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        rowOpts.forEach { (theme, label) ->
                            FilterChip(
                                selected = s.boardTheme == theme,
                                onClick = { store.update { it.copy(boardTheme = theme) } },
                                label = { Text(label) },
                                shape = RoundedCornerShape(14.dp),
                                modifier = Modifier.weight(1f)
                            )
                        }
                    }
                }
            }
        }

        ZenCard(Modifier.fillMaxWidth()) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                ToggleRow("Coordinates", s.showCoordinates) {
                    store.update { it.copy(showCoordinates = it.showCoordinates.not()) }
                }
                ToggleRow("Move numbers in review", s.showMoveNumbers) {
                    store.update { it.copy(showMoveNumbers = it.showMoveNumbers.not()) }
                }
                ToggleRow("Beginner hints", s.beginnerHints) {
                    store.update { it.copy(beginnerHints = it.beginnerHints.not()) }
                }
            }
        }

        Spacer(Modifier.height(8.dp))
        Text(
            "Weiqi · Local MVP",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun ToggleRow(label: String, checked: Boolean, onToggle: () -> Unit) {
    Row(
        Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium)
        Switch(checked = checked, onCheckedChange = { onToggle() })
    }
}
