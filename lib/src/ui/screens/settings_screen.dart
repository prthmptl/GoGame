import 'package:flutter/material.dart';

import '../../data/settings_store.dart';
import '../components/zen_components.dart';

class SettingsScreen extends StatefulWidget {
  final SettingsStore store;
  const SettingsScreen({super.key, required this.store});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  @override
  void initState() {
    super.initState();
    widget.store.addListener(_onChange);
  }

  @override
  void dispose() {
    widget.store.removeListener(_onChange);
    super.dispose();
  }

  void _onChange() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final s = widget.store.value;

    return SingleChildScrollView(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Settings',
              style:
                  text.headlineMedium?.copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 12),
          ZenCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('BOARD THEME',
                    style: text.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant)),
                const SizedBox(height: 8),
                _themeRow([
                  (BoardThemeKind.classicWood, 'Wood'),
                  (BoardThemeKind.minimalPaper, 'Paper'),
                ], s),
                const SizedBox(height: 8),
                _themeRow([
                  (BoardThemeKind.darkSlate, 'Slate'),
                  (BoardThemeKind.highContrast, 'High Contrast'),
                ], s),
              ],
            ),
          ),
          const SizedBox(height: 12),
          ZenCard(
            child: Column(
              children: [
                _ToggleRow(
                  label: 'Coordinates',
                  checked: s.showCoordinates,
                  onToggle: () => widget.store.update(
                      (v) => v.copyWith(showCoordinates: !v.showCoordinates)),
                ),
                _ToggleRow(
                  label: 'Move numbers in review',
                  checked: s.showMoveNumbers,
                  onToggle: () => widget.store.update(
                      (v) => v.copyWith(showMoveNumbers: !v.showMoveNumbers)),
                ),
                _ToggleRow(
                  label: 'Beginner hints',
                  checked: s.beginnerHints,
                  onToggle: () => widget.store.update(
                      (v) => v.copyWith(beginnerHints: !v.beginnerHints)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Text(
            'Weiqi · Local MVP',
            style: text.labelSmall?.copyWith(color: scheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  Widget _themeRow(List<(BoardThemeKind, String)> opts, AppSettings s) {
    return Row(
      children: [
        for (var i = 0; i < opts.length; i++) ...[
          Expanded(
            child:
                _themeChip(opts[i].$1, opts[i].$2, s.boardTheme == opts[i].$1),
          ),
          if (i != opts.length - 1) const SizedBox(width: 8),
        ]
      ],
    );
  }

  Widget _themeChip(BoardThemeKind theme, String label, bool selected) {
    final scheme = Theme.of(context).colorScheme;
    return FilterChip(
      selected: selected,
      onSelected: (_) =>
          widget.store.update((v) => v.copyWith(boardTheme: theme)),
      label: Text(label,
          style:
              TextStyle(color: selected ? scheme.onPrimary : scheme.onSurface)),
      backgroundColor: scheme.surfaceContainerHigh,
      selectedColor: scheme.primary,
      checkmarkColor: scheme.onPrimary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      side: BorderSide.none,
    );
  }
}

class _ToggleRow extends StatelessWidget {
  final String label;
  final bool checked;
  final VoidCallback onToggle;
  const _ToggleRow(
      {required this.label, required this.checked, required this.onToggle});

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: text.bodyMedium),
          Switch(value: checked, onChanged: (_) => onToggle()),
        ],
      ),
    );
  }
}
