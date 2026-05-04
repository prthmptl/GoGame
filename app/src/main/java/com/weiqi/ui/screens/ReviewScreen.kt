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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.weiqi.ui.components.ZenCard

@Composable
fun ReviewScreen() {
    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp)
    ) {
        ZenCard(
            modifier = Modifier.fillMaxWidth(),
            container = MaterialTheme.colorScheme.surfaceContainerLow
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Player One (7d)",
                        style = MaterialTheme.typography.bodyMedium)
                    Text("vs Player Two (6d)",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                com.weiqi.ui.components.ZenChip("REVIEW")
                Spacer(Modifier.padding(start = 12.dp))
                Text("Move 142/215", style = MaterialTheme.typography.bodyMedium)
            }
        }

        Spacer(Modifier.height(12.dp))

        // Empty board slot — real review would render the historical board.
        androidx.compose.foundation.layout.Box(
            Modifier
                .fillMaxWidth()
                .height(360.dp)
                .background(com.weiqi.ui.theme.Zen.kayaWood,
                    shape = RoundedCornerShape(4.dp))
        )

        Spacer(Modifier.height(16.dp))

        ZenCard(Modifier.fillMaxWidth()) {
            Column {
                Text("Win Probability", style = MaterialTheme.typography.titleLarge)
                Spacer(Modifier.height(8.dp))
                androidx.compose.foundation.layout.Box(
                    Modifier
                        .fillMaxWidth()
                        .height(120.dp)
                        .background(MaterialTheme.colorScheme.surfaceContainerLow,
                            shape = RoundedCornerShape(4.dp))
                )
                Spacer(Modifier.height(8.dp))
                Row(
                    Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text("Move 1", style = MaterialTheme.typography.labelSmall)
                    Text("W +12.4", style = MaterialTheme.typography.labelMedium)
                    Text("Move 215", style = MaterialTheme.typography.labelSmall)
                }
            }
        }

        Spacer(Modifier.height(12.dp))

        ZenCard(Modifier.fillMaxWidth()) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                Text("⏮", style = MaterialTheme.typography.titleLarge)
                Text("◀", style = MaterialTheme.typography.titleLarge)
                Text("▶", style = MaterialTheme.typography.titleLarge)
                Text("⏭", style = MaterialTheme.typography.titleLarge)
            }
        }

        Spacer(Modifier.height(24.dp))
    }
}
