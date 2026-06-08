import '../board.dart';
import '../bots/bot_profile.dart';
import '../game_state.dart';
import '../groups.dart';
import '../models.dart';
import '../rules.dart';
import 'ai_heuristics.dart';
import 'go_ai.dart';

class StyledAi implements GoAi {
  final GoAi base;
  final BotStyle style;

  const StyledAi({required this.base, required this.style});

  @override
  MoveIntent chooseMove(GameState state) {
    final styled = _styledMove(state);
    if (styled != null) return MoveIntent.place(styled);
    return base.chooseMove(state);
  }

  Point? _styledMove(GameState state) {
    if (style == BotStyle.balanced) return null;

    final legal = AiHeuristics.legalNonEyeMoves(state);
    if (legal.isEmpty) return null;

    final captures = AiHeuristics.capturingMoves(state, legal: legal);
    if (captures.isNotEmpty) {
      return _bestPoint(
        captures.map((m) => m.point),
        (p) => AiHeuristics.captureScore(
          captures.firstWhere((m) => m.point == p),
          state,
          state.currentPlayer,
        ),
      );
    }

    final saves = AiHeuristics.savingMoves(state, legal: legal);
    if (saves.isNotEmpty) {
      return _bestPoint(saves, (p) => _styleScore(state, p));
    }

    final safe = legal
        .where((p) => !AiHeuristics.isSelfAtari(state, p))
        .toList(growable: false);
    final pool = safe.isEmpty ? legal : safe;

    return _bestPoint(pool, (p) => _styleScore(state, p));
  }

  int _styleScore(GameState state, Point p) => switch (style) {
        BotStyle.aggressive => AiHeuristics.scoreCandidate(state, p) +
            _adjacency(state.board, p, state.currentPlayer.other) * 8,
        BotStyle.territorial => AiHeuristics.scoreCandidate(state, p) +
            (_isTerritoryBand(state.board, p) ? 12 : 0),
        BotStyle.calm => AiHeuristics.scoreCandidate(state, p) +
            _libertyBonusAfter(state, p) * 6,
        BotStyle.tricky => AiHeuristics.scoreCandidate(state, p) +
            _cutBonus(state, p) +
            _adjacency(state.board, p, state.currentPlayer.other) * 3,
        BotStyle.balanced => AiHeuristics.scoreCandidate(state, p),
      };

  Point? _bestPoint(Iterable<Point> points, int Function(Point) score) {
    Point? best;
    var bestScore = -1 << 30;
    for (final p in points) {
      final s = score(p);
      if (best == null ||
          s > bestScore ||
          (s == bestScore && _comparePoint(p, best) < 0)) {
        best = p;
        bestScore = s;
      }
    }
    return best;
  }

  static int _adjacency(Board board, Point p, StoneColor color) {
    final wanted = CellState.of(color);
    var count = 0;
    for (final n in board.neighbors(p)) {
      if (board.cellAt(n) == wanted) count++;
    }
    return count;
  }

  static int _edgeDistance(Board board, Point p) {
    final a = p.row < p.col ? p.row : p.col;
    final b = board.size - 1 - p.row;
    final c = board.size - 1 - p.col;
    final d = b < c ? b : c;
    return a < d ? a : d;
  }

  static bool _isTerritoryBand(Board board, Point p) {
    final edge = _edgeDistance(board, p);
    return edge == 2 || edge == 3;
  }

  static int _libertyBonusAfter(GameState state, Point p) {
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return -1 << 30;
    final next = res.newStateAs<GameState>();
    if (next.board.cellAt(p) == CellState.empty) return 0;
    return internalLiberties(next.board, p);
  }

  static int _cutBonus(GameState state, Point p) {
    final ownAdj = _adjacency(state.board, p, state.currentPlayer);
    final oppAdj = _adjacency(state.board, p, state.currentPlayer.other);
    return oppAdj >= 2 && ownAdj <= 1 ? 18 : 0;
  }

  static int _comparePoint(Point a, Point b) {
    final row = a.row.compareTo(b.row);
    if (row != 0) return row;
    return a.col.compareTo(b.col);
  }
}
