import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../theme.dart';

/// Compact line chart used for the review screen's "black lead" trend.
///
/// The center line is the even-game baseline; the line sits above it when
/// black is ahead and below it when white is ahead. An optional [cursor]
/// highlights the move currently being viewed.
class Sparkline extends StatelessWidget {
  final List<double> values;
  final int? cursor;
  final double maxAbs;

  const Sparkline({
    super.key,
    required this.values,
    this.cursor,
    this.maxAbs = 30,
  });

  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      painter: _SparklinePainter(
        values: values,
        cursor: cursor,
        maxAbs: maxAbs,
        ink: Theme.of(context).colorScheme.onSurfaceVariant,
        accent: Zen.primary,
      ),
      size: const Size.fromHeight(80),
    );
  }
}

class _SparklinePainter extends CustomPainter {
  final List<double> values;
  final int? cursor;
  final double maxAbs;
  final Color ink;
  final Color accent;

  _SparklinePainter({
    required this.values,
    required this.cursor,
    required this.maxAbs,
    required this.ink,
    required this.accent,
  });

  @override
  void paint(Canvas canvas, Size size) {
    if (values.isEmpty) return;
    final w = size.width;
    final h = size.height;
    final midY = h / 2;

    canvas.drawLine(
      Offset(0, midY),
      Offset(w, midY),
      Paint()
        ..color = ink.withValues(alpha: 0.25)
        ..strokeWidth = 1,
    );

    final clamp = math.max(maxAbs, 1.0);
    final stepX = values.length <= 1 ? w : w / (values.length - 1);
    final path = Path();
    for (var i = 0; i < values.length; i++) {
      final v = (values[i] / clamp).clamp(-1.0, 1.0);
      final x = i * stepX;
      final y = midY - v * (h / 2 - 4);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    canvas.drawPath(
      path,
      Paint()
        ..color = accent
        ..strokeWidth = 1.6
        ..style = PaintingStyle.stroke
        ..strokeJoin = StrokeJoin.round,
    );

    final idx = cursor;
    if (idx != null && idx >= 0 && idx < values.length) {
      final v = (values[idx] / clamp).clamp(-1.0, 1.0);
      final x = idx * stepX;
      final y = midY - v * (h / 2 - 4);
      canvas.drawLine(
        Offset(x, 0),
        Offset(x, h),
        Paint()
          ..color = accent.withValues(alpha: 0.25)
          ..strokeWidth = 1,
      );
      canvas.drawCircle(Offset(x, y), 3.5, Paint()..color = accent);
    }
  }

  @override
  bool shouldRepaint(covariant _SparklinePainter oldDelegate) =>
      oldDelegate.values != values ||
      oldDelegate.cursor != cursor ||
      oldDelegate.maxAbs != maxAbs;
}
