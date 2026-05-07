import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../domain/board.dart';
import '../../domain/models.dart';
import '../theme.dart';

class BoardOverlay {
  final Point? lastMove;
  final Point? koPoint;
  final Set<Point> deadStones;
  final Set<Point> territoryBlack;
  final Set<Point> territoryWhite;
  final Map<Point, int> moveNumbers;

  /// Ghost stone shown before the user confirms a move (beginner hints mode).
  final ({Point point, StoneColor color})? pending;

  /// Markers (small filled dots) — used for tutorial diagrams.
  final Set<Point> markers;

  const BoardOverlay({
    this.lastMove,
    this.koPoint,
    this.deadStones = const {},
    this.territoryBlack = const {},
    this.territoryWhite = const {},
    this.moveNumbers = const {},
    this.pending,
    this.markers = const {},
  });
}

class BoardAppearance {
  final Color board;
  final Color boardEdge;
  final Color ink;
  final bool showCoordinates;

  const BoardAppearance({
    this.board = Zen.kayaWood,
    this.boardEdge = Zen.kayaWoodEdge,
    this.ink = Zen.gridInk,
    this.showCoordinates = false,
  });

  BoardAppearance copyWith(
          {Color? board,
          Color? boardEdge,
          Color? ink,
          bool? showCoordinates}) =>
      BoardAppearance(
        board: board ?? this.board,
        boardEdge: boardEdge ?? this.boardEdge,
        ink: ink ?? this.ink,
        showCoordinates: showCoordinates ?? this.showCoordinates,
      );

  static const classicWood = BoardAppearance();
  static const minimalPaper = BoardAppearance(
    board: Color(0xFFFAF6EE),
    boardEdge: Color(0xFFE0DAC8),
    ink: Color(0xFF1A1A1A),
  );
  static const darkSlate = BoardAppearance(
    board: Color(0xFFD8D7D2),
    boardEdge: Color(0xFFB7B6B0),
    ink: Color(0xFF1A1A1A),
  );
  static const highContrast = BoardAppearance(
    board: Color(0xFFFFF1CD),
    boardEdge: Color(0xFFE6D496),
    ink: Color(0xFF000000),
  );
}

class _CaptureGhost {
  final Point point;
  final CellState color;
  const _CaptureGhost(this.point, this.color);
}

class BoardCanvas extends StatefulWidget {
  final Board board;
  final BoardOverlay overlay;
  final BoardAppearance appearance;
  final ValueChanged<Point>? onTap;

  const BoardCanvas({
    super.key,
    required this.board,
    this.overlay = const BoardOverlay(),
    this.appearance = BoardAppearance.classicWood,
    this.onTap,
  });

  @override
  State<BoardCanvas> createState() => _BoardCanvasState();
}

class _BoardCanvasState extends State<BoardCanvas>
    with TickerProviderStateMixin {
  late final AnimationController _placeController;
  late final AnimationController _captureController;
  Board? _prevBoard;
  Point? _prevLastMove;
  List<_CaptureGhost> _captureGhosts = const [];

  @override
  void initState() {
    super.initState();
    _placeController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 180),
      value: 1,
    );
    _captureController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 240),
      value: 1,
    );
    _prevBoard = widget.board;
    _prevLastMove = widget.overlay.lastMove;
  }

  @override
  void didUpdateWidget(covariant BoardCanvas oldWidget) {
    super.didUpdateWidget(oldWidget);
    final lastMove = widget.overlay.lastMove;
    if (lastMove != _prevLastMove) {
      if (lastMove != null &&
          widget.board.cellAt(lastMove) != CellState.empty) {
        _placeController.value = 0.55;
        _placeController.animateTo(1, curve: Curves.easeOut);
      } else {
        _placeController.value = 1;
      }
      _prevLastMove = lastMove;
    }
    if (!identical(_prevBoard, widget.board) &&
        _prevBoard != null &&
        _prevBoard!.size == widget.board.size) {
      final gone = <_CaptureGhost>[];
      for (var r = 0; r < widget.board.size; r++) {
        for (var c = 0; c < widget.board.size; c++) {
          final before = _prevBoard!.cellAtRC(r, c);
          final after = widget.board.cellAtRC(r, c);
          if (before != CellState.empty && after == CellState.empty) {
            gone.add(_CaptureGhost(Point(r, c), before));
          }
        }
      }
      if (gone.isNotEmpty) {
        _captureGhosts = gone;
        _captureController.value = 1;
        _captureController.animateTo(0).then((_) {
          if (!mounted) return;
          setState(() => _captureGhosts = const []);
        });
      }
    }
    _prevBoard = widget.board;
  }

  @override
  void dispose() {
    _placeController.dispose();
    _captureController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTapDown: (details) {
        final box = context.findRenderObject() as RenderBox?;
        if (box == null) return;
        final size = box.size;
        final side = math.min(size.width, size.height);
        final pad = side * (widget.appearance.showCoordinates ? 0.085 : 0.06);
        final usable = side - 2 * pad;
        final step = usable / (widget.board.size - 1);
        final col = ((details.localPosition.dx - pad) / step).round();
        final row = ((details.localPosition.dy - pad) / step).round();
        if (row >= 0 &&
            row < widget.board.size &&
            col >= 0 &&
            col < widget.board.size) {
          widget.onTap?.call(Point(row, col));
        }
      },
      child: AnimatedBuilder(
        animation: Listenable.merge([_placeController, _captureController]),
        builder: (context, _) => CustomPaint(
          size: Size.infinite,
          painter: _BoardPainter(
            board: widget.board,
            overlay: widget.overlay,
            appearance: widget.appearance,
            placeScale: _placeController.value,
            captureAlpha: _captureController.value,
            captureGhosts: _captureGhosts,
          ),
        ),
      ),
    );
  }
}

class _BoardPainter extends CustomPainter {
  final Board board;
  final BoardOverlay overlay;
  final BoardAppearance appearance;
  final double placeScale;
  final double captureAlpha;
  final List<_CaptureGhost> captureGhosts;

  _BoardPainter({
    required this.board,
    required this.overlay,
    required this.appearance,
    required this.placeScale,
    required this.captureAlpha,
    required this.captureGhosts,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final side = math.min(size.width, size.height);
    final pad = side * (appearance.showCoordinates ? 0.085 : 0.06);
    final usable = side - 2 * pad;
    final step = usable / (board.size - 1);
    final stoneR = step * 0.46;
    final ink20 = appearance.ink.withValues(alpha: 0.20);
    final ink70 = appearance.ink.withValues(alpha: 0.70);

    final shadowInset = side * 0.012;
    _drawSoftShadow(
      canvas,
      Offset(shadowInset, shadowInset * 1.4),
      Size(side - 2 * shadowInset, side - 2 * shadowInset),
    );

    final boardRect = Rect.fromLTWH(0, 0, side, side);
    final boardPaint = Paint()
      ..shader = LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [appearance.board, appearance.boardEdge],
      ).createShader(boardRect);
    canvas.drawRect(boardRect, boardPaint);
    canvas.drawRect(
      boardRect,
      Paint()
        ..style = PaintingStyle.stroke
        ..color = appearance.boardEdge.withValues(alpha: 0.35)
        ..strokeWidth = side * 0.012,
    );

    final gridPaint = Paint()
      ..color = ink20
      ..strokeWidth = 1.2;
    for (var i = 0; i < board.size; i++) {
      final pos = pad + step * i;
      canvas.drawLine(Offset(pad, pos), Offset(pad + usable, pos), gridPaint);
      canvas.drawLine(Offset(pos, pad), Offset(pos, pad + usable), gridPaint);
    }
    canvas.drawRect(
      Rect.fromLTWH(pad, pad, usable, usable),
      Paint()
        ..style = PaintingStyle.stroke
        ..color = ink20.withValues(alpha: 0.30)
        ..strokeWidth = 1.4,
    );
    for (final sp in _starPointsFor(board.size)) {
      canvas.drawCircle(
        Offset(pad + step * sp.col, pad + step * sp.row),
        step * 0.085,
        Paint()..color = ink70,
      );
    }

    if (appearance.showCoordinates) {
      final color = appearance.ink.withValues(alpha: 0.65);
      final fontSize = math.min(step * 0.32, 20.0);
      for (var i = 0; i < board.size; i++) {
        final colLetter = _colLetter(i);
        final rowLabel = (board.size - i).toString();
        final xCol = pad + step * i;
        final yRow = pad + step * i;
        final colTp = _measure(colLetter, color, fontSize);
        final rowTp = _measure(rowLabel, color, fontSize);
        colTp.paint(canvas,
            Offset(xCol - colTp.width / 2, pad - colTp.height - step * 0.10));
        colTp.paint(
            canvas, Offset(xCol - colTp.width / 2, pad + usable + step * 0.10));
        rowTp.paint(canvas,
            Offset(pad - rowTp.width - step * 0.18, yRow - rowTp.height / 2));
        rowTp.paint(canvas,
            Offset(pad + usable + step * 0.18, yRow - rowTp.height / 2));
      }
    }

    for (final p in overlay.territoryBlack) {
      canvas.drawCircle(
        Offset(pad + step * p.col, pad + step * p.row),
        stoneR * 0.40,
        Paint()..color = Colors.black.withValues(alpha: 0.22),
      );
    }
    for (final p in overlay.territoryWhite) {
      canvas.drawCircle(
        Offset(pad + step * p.col, pad + step * p.row),
        stoneR * 0.40,
        Paint()..color = Colors.white.withValues(alpha: 0.55),
      );
    }

    for (final p in overlay.markers) {
      canvas.drawCircle(
        Offset(pad + step * p.col, pad + step * p.row),
        stoneR * 0.30,
        Paint()..color = const Color(0xFFB23A2E).withValues(alpha: 0.85),
      );
    }

    if (captureGhosts.isNotEmpty) {
      for (final g in captureGhosts) {
        _drawStone(
          canvas,
          cellState: g.color,
          center: Offset(pad + step * g.point.col, pad + step * g.point.row),
          stoneR: stoneR,
          ink: appearance.ink,
          scale: 1,
          alpha: captureAlpha,
        );
      }
    }

    for (var r = 0; r < board.size; r++) {
      for (var c = 0; c < board.size; c++) {
        final state = board.cellAtRC(r, c);
        if (state == CellState.empty) continue;
        final center = Offset(pad + step * c, pad + step * r);
        final isLast = overlay.lastMove != null &&
            overlay.lastMove!.row == r &&
            overlay.lastMove!.col == c;
        _drawStone(
          canvas,
          cellState: state,
          center: center,
          stoneR: stoneR,
          ink: appearance.ink,
          scale: isLast ? placeScale : 1.0,
          alpha: 1,
        );
        if (overlay.deadStones.contains(Point(r, c))) {
          final s = stoneR * 0.55;
          final paint = Paint()
            ..color = const Color(0xFFB23A2E)
            ..strokeWidth = 3;
          canvas.drawLine(Offset(center.dx - s, center.dy - s),
              Offset(center.dx + s, center.dy + s), paint);
          canvas.drawLine(Offset(center.dx - s, center.dy + s),
              Offset(center.dx + s, center.dy - s), paint);
        }
        final n = overlay.moveNumbers[Point(r, c)];
        if (n != null) {
          final isBlack = state == CellState.black;
          final fontSize = math.min(stoneR * 0.85, 18.0);
          final tp = _measure(
              n.toString(), isBlack ? Colors.white : Colors.black, fontSize);
          tp.paint(canvas,
              Offset(center.dx - tp.width / 2, center.dy - tp.height / 2));
        }
      }
    }

    final pending = overlay.pending;
    if (pending != null && board.cellAt(pending.point) == CellState.empty) {
      _drawStone(
        canvas,
        cellState: pending.color == StoneColor.black
            ? CellState.black
            : CellState.white,
        center: Offset(
            pad + step * pending.point.col, pad + step * pending.point.row),
        stoneR: stoneR,
        ink: appearance.ink,
        scale: 1,
        alpha: 0.45,
      );
    }

    final lm = overlay.lastMove;
    if (lm != null && board.cellAt(lm) != CellState.empty) {
      final state = board.cellAt(lm);
      final ringColor = state == CellState.black ? Colors.white : Colors.black;
      canvas.drawCircle(
        Offset(pad + step * lm.col, pad + step * lm.row),
        stoneR * 0.32,
        Paint()
          ..style = PaintingStyle.stroke
          ..color = ringColor
          ..strokeWidth = 1.1,
      );
    }
    final ko = overlay.koPoint;
    if (ko != null) {
      canvas.drawCircle(
        Offset(pad + step * ko.col, pad + step * ko.row),
        stoneR * 0.28,
        Paint()
          ..style = PaintingStyle.stroke
          ..color = const Color(0xFFB23A2E)
          ..strokeWidth = 2,
      );
    }
  }

  void _drawStone(
    Canvas canvas, {
    required CellState cellState,
    required Offset center,
    required double stoneR,
    required Color ink,
    required double scale,
    required double alpha,
  }) {
    final r = stoneR * scale;
    canvas.drawCircle(
      Offset(center.dx + r * 0.05, center.dy + r * 0.18),
      r,
      Paint()..color = Colors.black.withValues(alpha: 0.25 * alpha),
    );
    final isBlack = cellState == CellState.black;
    final highlight = isBlack ? Zen.blackStoneTop : Zen.whiteStoneTop;
    final body = isBlack ? Zen.blackStoneBottom : Zen.whiteStoneBottom;
    final rect = Rect.fromCircle(center: center, radius: r);
    final shader = RadialGradient(
      center: const Alignment(-0.6, -0.6),
      radius: 0.95,
      colors: [
        highlight.withValues(alpha: alpha),
        body.withValues(alpha: alpha),
      ],
    ).createShader(rect);
    canvas.drawCircle(center, r, Paint()..shader = shader);
    canvas.drawCircle(
      center,
      r,
      Paint()
        ..style = PaintingStyle.stroke
        ..color = isBlack
            ? Colors.black.withValues(alpha: 0.82 * alpha)
            : ink.withValues(alpha: 0.42 * alpha)
        ..strokeWidth = isBlack ? 0.45 : 0.7,
    );
  }

  void _drawSoftShadow(Canvas canvas, Offset topLeft, Size sz) {
    const base = Color(0x1F000000);
    for (var i = 0; i < 3; i++) {
      final grow = (i + 1) * 4.0;
      canvas.drawRect(
        Rect.fromLTWH(
          topLeft.dx - grow,
          topLeft.dy - grow + 2,
          sz.width + grow * 2,
          sz.height + grow * 2,
        ),
        Paint()..color = base.withValues(alpha: base.a / (i + 2)),
      );
    }
  }

  TextPainter _measure(String text, Color color, double fontSize) {
    final tp = TextPainter(
      text: TextSpan(
          text: text, style: TextStyle(color: color, fontSize: fontSize)),
      textDirection: TextDirection.ltr,
    );
    tp.layout();
    return tp;
  }

  String _colLetter(int col) {
    const skipI = 8; // 'I' - 'A'
    final code = col < skipI ? 0x41 + col : 0x41 + col + 1;
    return String.fromCharCode(code);
  }

  List<Point> _starPointsFor(int size) {
    if (size != 9 && size != 13 && size != 19) return const [];
    final edge = size == 9 ? 2 : 3;
    final far = size - 1 - edge;
    final mid = size ~/ 2;
    if (size == 9 || size == 13) {
      return [
        Point(edge, edge),
        Point(edge, far),
        Point(far, edge),
        Point(far, far),
        Point(mid, mid),
      ];
    }
    return [
      Point(edge, edge),
      Point(edge, mid),
      Point(edge, far),
      Point(mid, edge),
      Point(mid, mid),
      Point(mid, far),
      Point(far, edge),
      Point(far, mid),
      Point(far, far),
    ];
  }

  @override
  bool shouldRepaint(covariant _BoardPainter oldDelegate) =>
      oldDelegate.board != board ||
      oldDelegate.overlay != overlay ||
      oldDelegate.appearance != appearance ||
      oldDelegate.placeScale != placeScale ||
      oldDelegate.captureAlpha != captureAlpha ||
      oldDelegate.captureGhosts != captureGhosts;
}
