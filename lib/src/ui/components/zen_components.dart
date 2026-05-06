import 'package:flutter/material.dart';

/// Paper-tone card. Soft 14dp rounding for the "carved wood" feel.
class ZenCard extends StatelessWidget {
  final Widget child;
  final Color? container;
  final EdgeInsetsGeometry contentPadding;
  final VoidCallback? onTap;

  const ZenCard({
    super.key,
    required this.child,
    this.container,
    this.contentPadding = const EdgeInsets.all(16),
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bg = container ?? scheme.surfaceContainer;
    final card = Material(
      color: bg,
      borderRadius: BorderRadius.circular(14),
      clipBehavior: Clip.antiAlias,
      child: onTap == null
          ? Padding(padding: contentPadding, child: child)
          : InkWell(
              onTap: onTap,
              child: Padding(padding: contentPadding, child: child),
            ),
    );
    return card;
  }
}

/// Pill-shaped tag. Picks a contrasting text color automatically based on the container.
class ZenChip extends StatelessWidget {
  final String text;
  final Color? container;
  final Color? contentColor;

  const ZenChip({
    super.key,
    required this.text,
    this.container,
    this.contentColor,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bg = container ?? scheme.secondaryContainer;
    final fg = contentColor ?? _contentColorFor(bg);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        text,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(color: fg),
      ),
    );
  }

  static Color _contentColorFor(Color bg) {
    final l = 0.299 * bg.r + 0.587 * bg.g + 0.114 * bg.b;
    return l < 0.5 ? const Color(0xFFFFF8F3) : const Color(0xFF221A10);
  }
}
