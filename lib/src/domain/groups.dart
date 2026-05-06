import 'board.dart';
import 'models.dart';

class StoneGroup {
  final Set<Point> stones;
  final Set<Point> liberties;
  StoneGroup(this.stones, this.liberties);
}

/// Liberty count of the group containing [start]. Returns 0 if [start] is empty.
int internalLiberties(Board board, Point start) {
  if (board.cellAt(start) == CellState.empty) return 0;
  return findGroup(board, start).liberties.length;
}

StoneGroup findGroup(Board board, Point start) {
  final color = board.cellAt(start);
  if (color == CellState.empty) {
    throw ArgumentError('findGroup called on empty point');
  }
  final stones = <Point>{};
  final libs = <Point>{};
  final stack = <Point>[start];
  while (stack.isNotEmpty) {
    final p = stack.removeLast();
    if (!stones.add(p)) continue;
    for (final n in board.neighbors(p)) {
      final s = board.cellAt(n);
      if (s == CellState.empty) {
        libs.add(n);
      } else if (s == color && !stones.contains(n)) {
        stack.add(n);
      }
    }
  }
  return StoneGroup(stones, libs);
}
