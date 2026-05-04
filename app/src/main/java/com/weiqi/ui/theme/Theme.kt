package com.weiqi.ui.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

/** Zen Go palette — "Digital Ink on Physical Wood." */
object Zen {
    val surface = Color(0xFFFFF8F3)
    val surfaceDim = Color(0xFFE7D8C6)
    val surfaceContainerLow = Color(0xFFFFF2E3)
    val surfaceContainer = Color(0xFFFBECDA)
    val surfaceContainerHigh = Color(0xFFF5E6D4)
    val surfaceContainerHighest = Color(0xFFEFE0CF)
    val onSurface = Color(0xFF221A10)
    val onSurfaceVariant = Color(0xFF504442)
    val outline = Color(0xFF827471)
    val outlineVariant = Color(0xFFD4C3BF)

    val primary = Color(0xFF361F1A)         // Walnut / Ink Black
    val onPrimary = Color(0xFFFFFFFF)
    val primaryContainer = Color(0xFF4E342E)
    val onPrimaryContainer = Color(0xFFC19C94)

    val secondary = Color(0xFF5F5E5F)       // Silt
    val secondaryContainer = Color(0xFFE2DFE0)
    val onSecondaryContainer = Color(0xFF636263)

    val tertiary = Color(0xFF242523)
    val tertiaryContainer = Color(0xFF3A3B39)
    val error = Color(0xFFBA1A1A)

    // Custom: board / stones.
    val kayaWood = Color(0xFFE8C99B)        // raised board
    val kayaWoodEdge = Color(0xFFCFA976)
    val tableShadow = Color(0x1F000000)
    val gridInk = Color(0xFF221A10)         // 20% opacity in renderer
    val blackStoneTop = Color(0xFF2B2B2B)
    val blackStoneBottom = Color(0xFF101010)
    val whiteStoneTop = Color(0xFFFFFCF6)
    val whiteStoneBottom = Color(0xFFE8DFD0)
}

private val ZenColors = lightColorScheme(
    primary = Zen.primary,
    onPrimary = Zen.onPrimary,
    primaryContainer = Zen.primaryContainer,
    onPrimaryContainer = Zen.onPrimaryContainer,
    secondary = Zen.secondary,
    onSecondary = Color.White,
    secondaryContainer = Zen.secondaryContainer,
    onSecondaryContainer = Zen.onSecondaryContainer,
    tertiary = Zen.tertiary,
    error = Zen.error,
    background = Zen.surface,
    onBackground = Zen.onSurface,
    surface = Zen.surface,
    onSurface = Zen.onSurface,
    surfaceVariant = Zen.surfaceContainerHighest,
    onSurfaceVariant = Zen.onSurfaceVariant,
    outline = Zen.outline,
    outlineVariant = Zen.outlineVariant,
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Zen.surfaceContainerLow,
    surfaceContainer = Zen.surfaceContainer,
    surfaceContainerHigh = Zen.surfaceContainerHigh,
    surfaceContainerHighest = Zen.surfaceContainerHighest,
)

private val Serif = FontFamily.Serif      // Falls back gracefully without bundled Noto Serif.
private val Sans = FontFamily.SansSerif   // Stand-in for Manrope.

private val ZenTypography = Typography(
    displayLarge = TextStyle(
        fontFamily = Serif, fontWeight = FontWeight.Bold,
        fontSize = 42.sp, lineHeight = 52.sp, letterSpacing = (-0.02).em
    ),
    headlineMedium = TextStyle(
        fontFamily = Serif, fontWeight = FontWeight.SemiBold,
        fontSize = 24.sp, lineHeight = 32.sp
    ),
    headlineSmall = TextStyle(
        fontFamily = Serif, fontWeight = FontWeight.SemiBold,
        fontSize = 20.sp, lineHeight = 28.sp
    ),
    titleLarge = TextStyle(
        fontFamily = Serif, fontWeight = FontWeight.SemiBold,
        fontSize = 22.sp, lineHeight = 28.sp
    ),
    bodyLarge = TextStyle(
        fontFamily = Sans, fontWeight = FontWeight.Normal,
        fontSize = 18.sp, lineHeight = 28.sp
    ),
    bodyMedium = TextStyle(
        fontFamily = Sans, fontWeight = FontWeight.Normal,
        fontSize = 16.sp, lineHeight = 24.sp
    ),
    labelLarge = TextStyle(
        fontFamily = Sans, fontWeight = FontWeight.SemiBold,
        fontSize = 14.sp, lineHeight = 20.sp, letterSpacing = 0.05.em
    ),
    labelMedium = TextStyle(
        fontFamily = Sans, fontWeight = FontWeight.SemiBold,
        fontSize = 14.sp, lineHeight = 20.sp, letterSpacing = 0.05.em
    ),
    labelSmall = TextStyle(
        fontFamily = Sans, fontWeight = FontWeight.Medium,
        fontSize = 12.sp, lineHeight = 16.sp, letterSpacing = 0.08.em
    ),
)

// Softer rounding throughout — gives a warm, hand-carved feel without becoming bubbly.
private val ZenShapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(12.dp),
    medium = RoundedCornerShape(14.dp),
    large = RoundedCornerShape(18.dp),
    extraLarge = RoundedCornerShape(24.dp),
)

@Composable
fun WeiqiTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = ZenColors,
        typography = ZenTypography,
        shapes = ZenShapes,
        content = content
    )
}
