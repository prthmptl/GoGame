import 'board.dart';
import 'game_state.dart';
import 'models.dart';

class ScoreResult {
  final int blackStones;
  final int whiteStones;
  final int blackTerritory;
  final int whiteTerritory;
  final int blackPrisoners;
  final int whitePrisoners;
  final int neutral;
  final double komi;
  final ScoringMethod method;

  const ScoreResult({
    required this.blackStones,
    required this.whiteStones,
    required this.blackTerritory,
    required this.whiteTerritory,
    required this.blackPrisoners,
    required this.whitePrisoners,
    required this.neutral,
    required this.komi,
    required this.method,
  });

  /// Legacy alias: living stones + surrounded territory (Chinese-style raw total).
  int get blackArea => blackStones + blackTerritory;
  int get whiteArea => whiteStones + whiteTerritory;

  double get blackTotal => method == ScoringMethod.area
      ? (blackStones + blackTerritory).toDouble()
      : (blackTerritory + blackPrisoners).toDouble();

  double get whiteTotal => method == ScoringMethod.area
      ? (whiteStones + whiteTerritory) + komi
      : (whiteTerritory + whitePrisoners) + komi;

  /// Positive = black wins by N. Negative = white wins by |N|.
  double get margin => blackTotal - whiteTotal;

  String get resultString {
    if (margin > 0) return 'B+${margin.toStringAsFixed(1)}';
    if (margin < 0) return 'W+${(-margin).toStringAsFixed(1)}';
    return 'Draw';
  }
}

class Scoring {
  /// Score the given state using the ruleset configured on the game.
  /// [deadStones] are removed before counting; under territory scoring,
  /// they are also added to the capturing player's prisoner total.
  static ScoreResult score(GameState state,
      {Set<Point> deadStones = const {}}) {
    final method = RulesetDefaults.of(state.config.ruleset).scoringMethod;

    // Tally dead-stone prisoners (used only under territory scoring).
    var deadBlack = 0;
    var deadWhite = 0;
    for (final p in deadStones) {
      switch (state.board.cellAt(p)) {
        case CellState.black:
          deadBlack++;
          break;
        case CellState.white:
          deadWhite++;
          break;
        case CellState.empty:
          break;
      }
    }

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

    // Prisoners: stones captured during play + opponent's dead stones at endgame.
    // Black's prisoners are white stones black has captured.
    final blackPrisoners = state.capturesByBlack + deadWhite;
    final whitePrisoners = state.capturesByWhite + deadBlack;

    return ScoreResult(
      blackStones: blackStones,
      whiteStones: whiteStones,
      blackTerritory: blackTerritory,
      whiteTerritory: whiteTerritory,
      blackPrisoners: blackPrisoners,
      whitePrisoners: whitePrisoners,
      neutral: neutral,
      komi: state.config.komi,
      method: method,
    );
  }

  static Board _removeDead(Board board, Set<Point> dead) {
    if (dead.isEmpty) return board;
    return board.setMany(dead.map((p) => MapEntry(p, CellState.empty)));
  }
}
