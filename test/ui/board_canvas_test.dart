import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/board.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/ui/board/board_canvas.dart';

void main() {
  for (final size in [9, 13, 19]) {
    testWidgets('$size board maps a centre tap and rejects drag gestures',
        (tester) async {
      final taps = <Point>[];
      await tester.pumpWidget(MaterialApp(
          home: Center(
              child: SizedBox(
                  width: 320,
                  height: 320,
                  child: BoardCanvas(
                      board: Board.empty(size), onTap: taps.add)))));
      final board = find.byType(BoardCanvas);
      await tester.tapAt(tester.getCenter(board));
      expect(taps, [Point(size ~/ 2, size ~/ 2)]);
      taps.clear();
      await tester.drag(board, const Offset(80, 0));
      expect(taps, isEmpty);
      expect(tester.takeException(), isNull);
    });
  }
}
