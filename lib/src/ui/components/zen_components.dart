import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../theme.dart';

/// Paper-tone card with a restrained shared corner radius.
class ZenCard extends StatefulWidget {
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
  State<ZenCard> createState() => _ZenCardState();
}

class _ZenCardState extends State<ZenCard> {
  bool _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bg = widget.container ?? scheme.surfaceContainer;
    final radius = BorderRadius.circular(Zen.cardRadius);
    final inner = Material(
      color: bg,
      borderRadius: radius,
      clipBehavior: Clip.antiAlias,
      child: widget.onTap == null
          ? Padding(padding: widget.contentPadding, child: widget.child)
          : InkWell(
              onTap: () {
                HapticFeedback.lightImpact();
                widget.onTap!();
              },
              child:
                  Padding(padding: widget.contentPadding, child: widget.child),
            ),
    );
    final card = DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: radius,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.04),
            blurRadius: 8,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: inner,
    );
    if (widget.onTap == null) return card;
    return Listener(
      onPointerDown: (_) => setState(() => _pressed = true),
      onPointerUp: (_) => setState(() => _pressed = false),
      onPointerCancel: (_) => setState(() => _pressed = false),
      child: AnimatedScale(
        scale: _pressed ? 0.97 : 1.0,
        duration: const Duration(milliseconds: 120),
        curve: Curves.easeOut,
        child: card,
      ),
    );
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

class ZenOptionButton extends StatefulWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const ZenOptionButton({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  State<ZenOptionButton> createState() => _ZenOptionButtonState();
}

class _ZenOptionButtonState extends State<ZenOptionButton> {
  bool _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bg = widget.selected ? scheme.primary : scheme.surfaceContainerHigh;
    final fg = widget.selected ? scheme.onPrimary : scheme.onSurface;
    final button = Material(
      color: bg,
      borderRadius: BorderRadius.circular(Zen.controlRadius),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () {
          HapticFeedback.selectionClick();
          widget.onTap();
        },
        child: SizedBox(
          height: 44,
          child: Center(
            child: Text(
              widget.label,
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
              style: Theme.of(context)
                  .textTheme
                  .labelMedium
                  ?.copyWith(color: fg),
            ),
          ),
        ),
      ),
    );
    return Listener(
      onPointerDown: (_) => setState(() => _pressed = true),
      onPointerUp: (_) => setState(() => _pressed = false),
      onPointerCancel: (_) => setState(() => _pressed = false),
      child: AnimatedScale(
        scale: _pressed ? 0.97 : 1.0,
        duration: const Duration(milliseconds: 120),
        curve: Curves.easeOut,
        child: button,
      ),
    );
  }
}
