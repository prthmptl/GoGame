import 'dart:math' as math;

import '../board.dart';
import '../game_state.dart';
import '../groups.dart';
import '../models.dart';
import '../rules.dart';

typedef AppliedMove = ({Point point, MoveResult result});

class AiHeuristics {
  const AiHeuristics._();

  static Point? starPoint(int size) {
    switch (size) {
      case 9:
        return const Point(4, 4);
      case 13:
      case 19:
        return const Point(3, 3);
      default:
        return null;
    }
  }

  static List<Point> legalNonEyeMoves(GameState state) => Rules
      .legalPlacements(state)
      .where((p) => !isOwnEye(state.board, p, state.currentPlayer))
      .toList(growable: false);

  static List<AppliedMove> capturingMoves(GameState state,
      {List<Point>? legal}) {
    final points = legal ?? legalNonEyeMoves(state);
    final out = <AppliedMove>[];
    for (final p in points) {
      final res = Rules.apply(state, MoveIntent.place(p));
      if (res.isAccepted && (res.move?.captured.isNotEmpty ?? false)) {
        out.add((point: p, result: res));
      }
    }
    return out;
  }

  static List<Point> savingMoves(GameState state, {List<Point>? legal}) =>
      (legal ?? legalNonEyeMoves(state))
          .where((p) => savesAtari(state, p))
          .toList(growable: false);

  static bool savesAtari(GameState state, Point p) {
    final board = state.board;
    final player = state.currentPlayer;
    final ownState = CellState.of(player);
    final ownInAtari = board.neighbors(p).any((n) =>
        board.cellAt(n) == ownState && internalLiberties(board, n) == 1);
    if (!ownInAtari) return false;
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return false;
    if ((res.move?.captured.isNotEmpty ?? false)) return true;
    final newState = res.newStateAs<GameState>();
    return newState.board.cellAt(p) != CellState.empty &&
        internalLiberties(newState.board, p) >= 2;
  }

  static bool isSelfAtari(GameState state, Point p) {
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return false;
    if ((res.move?.captured.isNotEmpty ?? false)) return false;
    final newState = res.newStateAs<GameState>();
    return newState.board.cellAt(p) != CellState.empty &&
        internalLiberties(newState.board, p) <= 1;
  }

  static bool isOwnEye(Board board, Point p, StoneColor player) {
    if (board.cellAt(p) != CellState.empty) return false;
    final own = CellState.of(player);
    final neighbors = board.neighbors(p);
    if (neighbors.any((n) => board.cellAt(n) != own)) return false;
    var bad = 0;
    final size = board.size;
    const diagonals = <(int, int)>[(-1, -1), (-1, 1), (1, -1), (1, 1)];
    for (final d in diagonals) {
      final nr = p.row + d.$1;
      final nc = p.col + d.$2;
      if (nr < 0 || nr >= size || nc < 0 || nc >= size) continue;
      if (board.cellAt(Point(nr, nc)) != own) bad++;
    }
    final onEdge =
        p.row == 0 || p.col == 0 || p.row == size - 1 || p.col == size - 1;
    return onEdge ? bad == 0 : bad <= 1;
  }

  static int captureScore(
      AppliedMove move, GameState state, StoneColor perspective) {
    final captured = move.result.move?.captured.length ?? 0;
    final next = move.result.newStateAs<GameState>();
    return captured * 24 +
        localMoveScore(
          move.point,
          state.board,
          perspective,
          perspective.other,
          state.moveNumber,
          state.lastMove?.point,
        ) +
        evaluate(next, perspective);
  }

  static int localMoveScore(
    Point p,
    Board board,
    StoneColor player,
    StoneColor opponent,
    int moveNumber,
    Point? lastOpponentMove,
  ) {
    var score = 0;
    final size = board.size;
    final ownState = CellState.of(player);
    final oppState = CellState.of(opponent);

    final edgeDist =
        [p.row, p.col, size - 1 - p.row, size - 1 - p.col].reduce(math.min);
    if (moveNumber < size * 2) {
      score += switch (edgeDist) {
        0 => -4,
        1 => -1,
        2 || 3 => 3,
        _ => 1,
      };
    }

    var ownAdj = 0;
    var oppAdj = 0;
    for (final n in board.neighbors(p)) {
      final s = board.cellAt(n);
      if (s == ownState) {
        ownAdj++;
      } else if (s == oppState) {
        oppAdj++;
      }
    }
    score += ownAdj * 2;
    score += oppAdj * 3;

    if (ownAdj >= 2) score += 2; // connection bias
    if (oppAdj >= 2) score += 2; // cut / pressure bias

    if (lastOpponentMove != null) {
      final d = (p.row - lastOpponentMove.row).abs() +
          (p.col - lastOpponentMove.col).abs();
      if (d <= 2) {
        score += 4;
      } else if (d <= 4) {
        score += 1;
      }
    }
    return score;
  }

  static int scoreCandidate(GameState state, Point p) {
    final player = state.currentPlayer;
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return -1 << 30;
    final next = res.newStateAs<GameState>();
    var score = localMoveScore(
      p,
      state.board,
      player,
      player.other,
      state.moveNumber,
      state.lastMove?.point,
    );
    score += (res.move?.captured.length ?? 0) * 24;
    if (next.board.cellAt(p) != CellState.empty) {
      score += math.min(internalLiberties(next.board, p), 4) * 3;
    }
    if (isSelfAtari(state, p)) score -= 24;
    score += evaluate(next, player) ~/ 6;
    return score;
  }

  static List<Point> candidateMoves(GameState state, {required int limit}) {
    final legal = legalNonEyeMoves(state);
    if (legal.isEmpty) return const [];
    final scored = legal
        .map((p) => (point: p, score: scoreCandidate(state, p)))
        .toList(growable: false)
      ..sort((a, b) => b.score.compareTo(a.score));
    return scored.take(limit).map((e) => e.point).toList(growable: false);
  }

  static int evaluate(GameState state, StoneColor perspective) {
    final board = state.board;
    var ownStones = 0;
    var oppStones = 0;
    var influence = 0;
    final own = CellState.of(perspective);
    final opp = CellState.of(perspective.other);

    for (var r = 0; r < board.size; r++) {
      for (var c = 0; c < board.size; c++) {
        final p = Point(r, c);
        final cell = board.cellAt(p);
        if (cell == own) ownStones++;
        if (cell == opp) oppStones++;
        if (cell == CellState.empty) {
          var ownAdj = 0;
          var oppAdj = 0;
          for (final n in board.neighbors(p)) {
            final s = board.cellAt(n);
            if (s == own) ownAdj++;
            if (s == opp) oppAdj++;
          }
          influence += ownAdj - oppAdj;
        }
      }
    }

    final ownCaps = perspective == StoneColor.black
        ? state.capturesByBlack
        : state.capturesByWhite;
    final oppCaps = perspective == StoneColor.black
        ? state.capturesByWhite
        : state.capturesByBlack;

    return (ownStones - oppStones) * 4 +
        (ownCaps - oppCaps) * 18 +
        _groupHealth(board, perspective) -
        _groupHealth(board, perspective.other) +
        influence;
  }

  static int _groupHealth(Board board, StoneColor color) {
    final wanted = CellState.of(color);
    final seen = <Point>{};
    var score = 0;
    for (var r = 0; r < board.size; r++) {
      for (var c = 0; c < board.size; c++) {
        final p = Point(r, c);
        if (board.cellAt(p) != wanted || seen.contains(p)) continue;
        final group = findGroup(board, p);
        seen.addAll(group.stones);
        final libs = group.liberties.length;
        final cappedLiberties = libs < 4 ? libs : 4;
        score += group.stones.length * cappedLiberties * 3;
        if (libs == 1) {
          score -= group.stones.length * 18;
        } else if (libs == 2) {
          score -= group.stones.length * 5;
        }
      }
    }
    return score;
  }
}

class AiSearch {
  const AiSearch._();

  static int minimax(
    GameState state,
    StoneColor root, {
    required int depth,
    required int width,
    int alpha = -1 << 30,
    int beta = 1 << 30,
  }) {
    if (depth <= 0 || state.status != GameStatus.active) {
      return AiHeuristics.evaluate(state, root);
    }

    final candidates = AiHeuristics.candidateMoves(state, limit: width);
    if (candidates.isEmpty) return AiHeuristics.evaluate(state, root);

    final maximizing = state.currentPlayer == root;
    var best = maximizing ? -1 << 30 : 1 << 30;

    for (final p in candidates) {
      final res = Rules.apply(state, MoveIntent.place(p));
      if (!res.isAccepted) continue;
      final next = res.newStateAs<GameState>();
      final value = minimax(
        next,
        root,
        depth: depth - 1,
        width: width,
        alpha: alpha,
        beta: beta,
      );

      if (maximizing) {
        best = math.max(best, value);
        alpha = math.max(alpha, best);
      } else {
        best = math.min(best, value);
        beta = math.min(beta, best);
      }
      if (beta <= alpha) break;
    }
    return best;
  }
}
