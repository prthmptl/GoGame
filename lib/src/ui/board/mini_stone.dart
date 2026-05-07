import 'package:flutter/material.dart';

import '../../domain/models.dart';
import '../theme.dart';

/// Small standalone stone, e.g. for player avatars in lists.
class MiniStone extends StatelessWidget {
  final StoneColor color;
  final double size;

  const MiniStone({super.key, required this.color, this.size = 24});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: CustomPaint(painter: _MiniStonePainter(color)),
    );
  }
}

class _MiniStonePainter extends CustomPainter {
  final StoneColor color;
  _MiniStonePainter(this.color);

  @override
  void paint(Canvas canvas, Size size) {
    final r = size.shortestSide / 2;
    final center = Offset(size.width / 2, size.height / 2);
    final isBlack = color == StoneColor.black;
    final top = isBlack ? Zen.blackStoneTop : Zen.whiteStoneTop;
    final bot = isBlack ? Zen.blackStoneBottom : Zen.whiteStoneBottom;

    canvas.drawCircle(
      center.translate(0, r * 0.18),
      r,
      Paint()..color = const Color(0x33000000),
    );

    final gradient = RadialGradient(
      center: const Alignment(-0.3, -0.3),
      radius: 0.7,
      colors: [top, bot],
    );
    final rect = Rect.fromCircle(center: center, radius: r);
    canvas.drawCircle(
      center,
      r,
      Paint()..shader = gradient.createShader(rect),
    );

    canvas.drawCircle(
      center,
      r,
      Paint()
        ..style = PaintingStyle.stroke
        ..color = isBlack
            ? Colors.black.withValues(alpha: 0.80)
            : Zen.gridInk.withValues(alpha: 0.42)
        ..strokeWidth = isBlack ? 0.45 : 0.7,
    );
  }

  @override
  bool shouldRepaint(covariant _MiniStonePainter oldDelegate) =>
      oldDelegate.color != color;
}
