import 'board.dart';
import 'game_state.dart';
import 'models.dart';

class ScoreResult {
  final int blackStones;
  final int whiteStones;
  final int blackTerritory;
  final int whiteTerritory;
  final int neutral;
  final double komi;

  const ScoreResult({
    required this.blackStones,
    required this.whiteStones,
    required this.blackTerritory,
    required this.whiteTerritory,
    required this.neutral,
    required this.komi,
  });

  int get blackArea => blackStones + blackTerritory;
  double get whiteTotal => (whiteStones + whiteTerritory) + komi;

  /// Positive = black wins by N. Negative = white wins by |N|.
  double get margin => blackArea - whiteTotal;

  String get resultString {
    if (margin > 0) return 'B+${margin.toStringAsFixed(1)}';
    if (margin < 0) return 'W+${(-margin).toStringAsFixed(1)}';
    return 'Draw';
  }
}

class Scoring {
  /// Chinese area scoring. [deadStones] are removed before counting.
  static ScoreResult score(GameState state,
      {Set<Point> deadStones = const {}}) {
    final board = _removeDead(state.board, deadStones);
    final size = board.size;

    var blackStones = 0;
    var whiteStones = 0;
    for (var r = 0; r < size; r++) {
      for (var c = 0; c < size; c++) {
        switch (board.cellAtRC(r, c)) {
          case CellState.black:
            blackStones++;
            break;
          case CellState.white:
            whiteStones++;
            break;
          case CellState.empty:
            break;
        }
      }
    }

    final visited = List<bool>.filled(size * size, false);
    var blackTerritory = 0;
    var whiteTerritory = 0;
    var neutral = 0;
    for (var r = 0; r < size; r++) {
      for (var c = 0; c < size; c++) {
        final idx = board.index(r, c);
        if (visited[idx]) continue;
        if (board.cellAtRC(r, c) != CellState.empty) {
          visited[idx] = true;
          continue;
        }
        // Flood-fill the empty region.
        final region = <Point>[];
        final borders = <CellState>{};
        final stack = <Point>[Point(r, c)];
        while (stack.isNotEmpty) {
          final p = stack.removeLast();
          final pi = board.indexOf(p);
          if (visited[pi]) continue;
          visited[pi] = true;
          region.add(p);
          for (final n in board.neighbors(p)) {
            final st = board.cellAt(n);
            if (st == CellState.empty) {
              if (!visited[board.indexOf(n)]) stack.add(n);
            } else {
              borders.add(st);
            }
          }
        }
        if (borders.length == 1 && borders.contains(CellState.black)) {
          blackTerritory += region.length;
        } else if (borders.length == 1 && borders.contains(CellState.white)) {
          whiteTerritory += region.length;
        } else {
          neutral += region.length;
        }
      }
    }

    return ScoreResult(
      blackStones: blackStones,
      whiteStones: whiteStones,
      blackTerritory: blackTerritory,
      whiteTerritory: whiteTerritory,
      neutral: neutral,
      komi: state.config.komi,
    );
  }

  static Board _removeDead(Board board, Set<Point> dead) {
    if (dead.isEmpty) return board;
    return board.setMany(dead.map((p) => MapEntry(p, CellState.empty)));
  }
}
