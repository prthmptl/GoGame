import 'package:flutter/material.dart';

/// "Digital Ink on Physical Wood" — Zen palette and typography.
class Zen {
  static const double cardRadius = 18;
  static const double controlRadius = 14;
  static const double dialogRadius = 20;

  static const surface = Color(0xFFFFF8F3);
  static const surfaceDim = Color(0xFFE7D8C6);
  static const surfaceContainerLow = Color(0xFFFFF2E3);
  static const surfaceContainer = Color(0xFFFBECDA);
  static const surfaceContainerHigh = Color(0xFFF5E6D4);
  static const surfaceContainerHighest = Color(0xFFEFE0CF);
  static const onSurface = Color(0xFF221A10);
  static const onSurfaceVariant = Color(0xFF504442);
  static const outline = Color(0xFF827471);
  static const outlineVariant = Color(0xFFD4C3BF);

  static const primary = Color(0xFF361F1A);
  static const onPrimary = Color(0xFFFFFFFF);
  static const primaryContainer = Color(0xFF4E342E);
  static const onPrimaryContainer = Color(0xFFC19C94);

  static const secondary = Color(0xFF5F5E5F);
  static const secondaryContainer = Color(0xFFE2DFE0);
  static const onSecondaryContainer = Color(0xFF636263);

  static const tertiary = Color(0xFF242523);
  static const tertiaryContainer = Color(0xFF3A3B39);
  static const error = Color(0xFFBA1A1A);

  // Custom: board / stones.
  static const kayaWood = Color(0xFFE8C99B);
  static const kayaWoodEdge = Color(0xFFCFA976);
  static const tableShadow = Color(0x1F000000);
  static const gridInk = Color(0xFF221A10);
  static const blackStoneTop = Color(0xFF2B2B2B);
  static const blackStoneBottom = Color(0xFF101010);
  static const whiteStoneTop = Color(0xFFFFFCF6);
  static const whiteStoneBottom = Color(0xFFE8DFD0);
}

ThemeData buildWeiqiTheme() {
  const scheme = ColorScheme(
    brightness: Brightness.light,
    primary: Zen.primary,
    onPrimary: Zen.onPrimary,
    primaryContainer: Zen.primaryContainer,
    onPrimaryContainer: Zen.onPrimaryContainer,
    secondary: Zen.secondary,
    onSecondary: Colors.white,
    secondaryContainer: Zen.secondaryContainer,
    onSecondaryContainer: Zen.onSecondaryContainer,
    tertiary: Zen.tertiary,
    onTertiary: Colors.white,
    error: Zen.error,
    onError: Colors.white,
    surface: Zen.surface,
    onSurface: Zen.onSurface,
    surfaceContainerLowest: Colors.white,
    surfaceContainerLow: Zen.surfaceContainerLow,
    surfaceContainer: Zen.surfaceContainer,
    surfaceContainerHigh: Zen.surfaceContainerHigh,
    surfaceContainerHighest: Zen.surfaceContainerHighest,
    surfaceDim: Zen.surfaceDim,
    outline: Zen.outline,
    outlineVariant: Zen.outlineVariant,
  );

  const serif = 'serif';
  const sans = 'sans-serif';

  final textTheme = TextTheme(
    displayLarge: TextStyle(
      fontFamily: serif,
      fontWeight: FontWeight.bold,
      fontSize: 42,
      height: 52 / 42,
      letterSpacing: 0,
      color: scheme.onSurface,
    ),
    headlineMedium: TextStyle(
      fontFamily: serif,
      fontWeight: FontWeight.w600,
      fontSize: 24,
      height: 32 / 24,
      color: scheme.onSurface,
    ),
    headlineSmall: TextStyle(
      fontFamily: serif,
      fontWeight: FontWeight.w600,
      fontSize: 20,
      height: 28 / 20,
      color: scheme.onSurface,
    ),
    titleLarge: TextStyle(
      fontFamily: serif,
      fontWeight: FontWeight.w600,
      fontSize: 22,
      height: 28 / 22,
      color: scheme.onSurface,
    ),
    bodyLarge: TextStyle(
      fontFamily: sans,
      fontSize: 18,
      height: 28 / 18,
      color: scheme.onSurface,
    ),
    bodyMedium: TextStyle(
      fontFamily: sans,
      fontSize: 16,
      height: 24 / 16,
      color: scheme.onSurface,
    ),
    labelLarge: TextStyle(
      fontFamily: sans,
      fontWeight: FontWeight.w600,
      fontSize: 14,
      height: 20 / 14,
      letterSpacing: 0,
      color: scheme.onSurface,
    ),
    labelMedium: TextStyle(
      fontFamily: sans,
      fontWeight: FontWeight.w600,
      fontSize: 14,
      height: 20 / 14,
      letterSpacing: 0,
      color: scheme.onSurface,
    ),
    labelSmall: TextStyle(
      fontFamily: sans,
      fontWeight: FontWeight.w500,
      fontSize: 12,
      height: 16 / 12,
      letterSpacing: 0,
      color: scheme.onSurface,
    ),
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: Zen.surface,
    canvasColor: Zen.surface,
    textTheme: textTheme,
    appBarTheme: const AppBarTheme(
      backgroundColor: Zen.surface,
      foregroundColor: Zen.onSurface,
      centerTitle: true,
      elevation: 0,
      scrolledUnderElevation: 0,
    ),
    navigationBarTheme: const NavigationBarThemeData(
      backgroundColor: Zen.surfaceContainer,
      indicatorColor: Zen.surfaceContainerHigh,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: scheme.primary,
        foregroundColor: scheme.onPrimary,
        minimumSize: const Size(0, 54),
        padding: const EdgeInsets.symmetric(horizontal: 24),
        textStyle: textTheme.labelLarge?.copyWith(
          fontWeight: FontWeight.w700,
          letterSpacing: 0.3,
        ),
        shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(Zen.controlRadius)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: scheme.onSurface,
        side: BorderSide(color: scheme.outline.withValues(alpha: 0.4)),
        shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(Zen.controlRadius)),
      ),
    ),
    chipTheme: ChipThemeData(
      shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(Zen.controlRadius)),
      backgroundColor: scheme.surfaceContainerHigh,
      selectedColor: scheme.primary,
      labelStyle: textTheme.labelLarge,
    ),
    sliderTheme: SliderThemeData(
      activeTrackColor: scheme.primary,
      inactiveTrackColor: scheme.surfaceContainerHigh,
      thumbColor: scheme.primary,
    ),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith((s) =>
          s.contains(WidgetState.selected) ? scheme.primary : Colors.white),
      trackColor: WidgetStateProperty.resolveWith((s) =>
          s.contains(WidgetState.selected)
              ? scheme.primary.withValues(alpha: 0.5)
              : scheme.surfaceContainerHigh),
    ),
    cardTheme: CardThemeData(
      color: scheme.surfaceContainer,
      elevation: 0,
      shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(Zen.cardRadius)),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: scheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(Zen.dialogRadius)),
    ),
  );
}
