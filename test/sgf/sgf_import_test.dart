import 'package:flutter_test/flutter_test.dart';
import 'package:weiqi/src/domain/models.dart';
import 'package:weiqi/src/sgf/sgf_import.dart';

void main() {
  test('imports header and moves', () {
    const sgf = '(;GM[1]FF[4]SZ[9]KM[7.5]HA[0];B[dc];W[ed];B[];W[ef])';
    final s = SgfImport.import(sgf);
    expect(s.config.boardSize, 9);
    expect(s.config.komi, closeTo(7.5, 1e-9));
    expect(s.board.cellAt(const Point(2, 3)), CellState.black);
    expect(s.board.cellAt(const Point(3, 4)), CellState.white);
    expect(s.board.cellAt(const Point(5, 4)), CellState.white);
  });
}
