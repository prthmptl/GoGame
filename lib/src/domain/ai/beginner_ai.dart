import 'dart:math' as math;

import '../board.dart';
import '../game_state.dart';
import '../groups.dart';
import '../models.dart';
import '../rules.dart';

abstract class GoAi {
  MoveIntent chooseMove(GameState state);
}

/// Heuristic Go AI. Move-selection priorities (highest first):
///   1. Capture an opponent group in atari.
///   2. Save own group in atari (extend OR capture the attacker).
///   3. Avoid self-atari, never fill own eye.
///   4. Top-N candidates evaluated 1 ply deep against opponent's best capture/atari.
///   5. Local response bias near opponent's last move.
///   6. Empty-board opening on a star point.
class BeginnerAi implements GoAi {
  final math.Random _random;

  BeginnerAi({math.Random? random}) : _random = random ?? math.Random();

  @override
  MoveIntent chooseMove(GameState state) {
    final board = state.board;

    // (6) Opening on an empty board: star point.
    if (state.history.isEmpty) {
      final sp = _starPoint(board.size);
      if (sp != null) return MoveIntent.place(sp);
    }

    final legal = Rules.legalPlacements(state)
        .where((p) => !_isOwnEye(board, p, state.currentPlayer))
        .toList(growable: false);
    if (legal.isEmpty) return const MoveIntent.pass();

    final player = state.currentPlayer;
    final opponent = player.other;

    // (1) Captures.
    final captures = <(Point, MoveResult)>[];
    for (final p in legal) {
      final res = Rules.apply(state, MoveIntent.place(p));
      if (res.isAccepted && (res.move?.captured.isNotEmpty ?? false)) {
        captures.add((p, res));
      }
    }
    if (captures.isNotEmpty) {
      var best = captures.first;
      var bestScore = _captureScore(best, board, player, opponent, state);
      for (final c in captures.skip(1)) {
        final s = _captureScore(c, board, player, opponent, state);
        if (s > bestScore) {
          best = c;
          bestScore = s;
        }
      }
      return MoveIntent.place(best.$1);
    }

    // (2) Escape atari: extend OR capture the attacker.
    final saves =
        legal.where((p) => _savesAtari(state, p)).toList(growable: false);
    if (saves.isNotEmpty) {
      return MoveIntent.place(_pickBest(saves, board, player, state));
    }

    // (3) Drop self-atari moves unless they capture (already handled above).
    final safe =
        legal.where((p) => !_isSelfAtari(state, p)).toList(growable: false);
    final pool = safe.isEmpty ? legal : safe;

    // Heuristic pre-score, then 1-ply lookahead on the top N.
    final scored = pool
        .map((p) => (
              p,
              _scoreMove(p, board, player, opponent, state.moveNumber,
                  state.lastMove?.point)
            ))
        .toList(growable: false);
    final maxScore = scored.map((e) => e.$2).reduce(math.max);
    final topPool = scored
        .where((e) => e.$2 >= maxScore - 2)
        .map((e) => e.$1)
        .toList()
      ..shuffle(_random);
    final topN = topPool.take(8).toList(growable: false);

    final refined =
        topN.map((p) => (p, _lookahead1Ply(state, p))).toList(growable: false);
    final best = refined.map((e) => e.$2).reduce(math.max);
    final finalists = refined
        .where((e) => e.$2 >= best - 1)
        .map((e) => e.$1)
        .toList(growable: false);

    if (state.moveNumber > board.size * board.size &&
        maxScore <= 0 &&
        best <= 0) {
      return const MoveIntent.pass();
    }
    return MoveIntent.place(finalists[_random.nextInt(finalists.length)]);
  }

  int _captureScore((Point, MoveResult) c, Board board, StoneColor player,
      StoneColor opponent, GameState state) {
    final captured = c.$2.move?.captured.length ?? 0;
    return captured * 10 +
        _scoreMove(c.$1, board, player, opponent, state.moveNumber,
            state.lastMove?.point);
  }

  Point _pickBest(
      List<Point> candidates, Board board, StoneColor player, GameState state) {
    final last = state.lastMove?.point;
    var best = candidates.first;
    var bestScore =
        _scoreMove(best, board, player, player.other, state.moveNumber, last);
    for (final p in candidates.skip(1)) {
      final s =
          _scoreMove(p, board, player, player.other, state.moveNumber, last);
      if (s > bestScore) {
        best = p;
        bestScore = s;
      }
    }
    return best;
  }

  bool _savesAtari(GameState state, Point p) {
    final board = state.board;
    final player = state.currentPlayer;
    final ownState = CellState.of(player);
    final ownInAtari = board.neighbors(p).any(
        (n) => board.cellAt(n) == ownState && internalLiberties(board, n) == 1);
    if (!ownInAtari) return false;
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return false;
    if ((res.move?.captured.isNotEmpty ?? false)) return true;
    final newState = res.newStateAs<GameState>();
    return internalLiberties(newState.board, p) >= 2;
  }

  bool _isSelfAtari(GameState state, Point p) {
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return false;
    if ((res.move?.captured.isNotEmpty ?? false)) return false;
    final newState = res.newStateAs<GameState>();
    return internalLiberties(newState.board, p) <= 1;
  }

  bool _isOwnEye(Board board, Point p, StoneColor player) {
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

  int _lookahead1Ply(GameState state, Point p) {
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return -1 << 31;
    final after = res.newStateAs<GameState>();
    var score = (res.move?.captured.length ?? 0) * 10;
    score += _scoreMove(p, state.board, state.currentPlayer,
        state.currentPlayer.other, state.moveNumber, state.lastMove?.point);

    final oppLegal =
        Rules.legalPlacements(after).take(40).toList(growable: false);
    var worst = 0;
    for (final op in oppLegal) {
      final r = Rules.apply(after, MoveIntent.place(op));
      if (!r.isAccepted) continue;
      final capturedUs = r.move?.captured.length ?? 0;
      if (capturedUs > 0 && capturedUs * -10 < worst) worst = capturedUs * -10;
      final next = r.newStateAs<GameState>();
      final ourLibs = next.board.cellAt(p) != CellState.empty
          ? internalLiberties(next.board, p)
          : 0;
      if (ourLibs == 1 && worst > -2) worst = -2;
    }
    return score + worst;
  }

  int _scoreMove(Point p, Board board, StoneColor player, StoneColor opponent,
      int moveNumber, Point? lastOpponentMove) {
    var score = 0;
    final size = board.size;
    final ownState = CellState.of(player);
    final oppState = CellState.of(opponent);

    final edgeDist =
        [p.row, p.col, size - 1 - p.row, size - 1 - p.col].reduce(math.min);
    if (moveNumber < size * 2) {
      score += switch (edgeDist) {
        0 => -3,
        1 => -1,
        2 || 3 => 2,
        _ => 1,
      };
    }

    for (final n in board.neighbors(p)) {
      final s = board.cellAt(n);
      if (s == ownState) {
        score += 1;
      } else if (s == oppState) {
        score += 2;
      }
    }

    if (lastOpponentMove != null) {
      final d = (p.row - lastOpponentMove.row).abs() +
          (p.col - lastOpponentMove.col).abs();
      if (d <= 2) {
        score += 3;
      } else if (d <= 4) {
        score += 1;
      }
    }

    score += _random.nextInt(2);
    return score;
  }

  Point? _starPoint(int size) {
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
}
