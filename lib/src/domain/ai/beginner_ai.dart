import 'dart:math' as math;

import '../board.dart';
import '../game_state.dart';
import '../groups.dart';
import '../models.dart';
import '../rules.dart';
import 'ai_heuristics.dart';
import 'go_ai.dart';

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
      final sp = AiHeuristics.starPoint(board.size);
      if (sp != null &&
          Rules.apply(state, MoveIntent.place(sp)).isAccepted) {
        return MoveIntent.place(sp);
      }
    }

    final legal = AiHeuristics.legalNonEyeMoves(state);
    if (legal.isEmpty) return const MoveIntent.pass();

    final player = state.currentPlayer;
    final opponent = player.other;

    // (1) Captures.
    final captures = AiHeuristics.capturingMoves(state, legal: legal);
    if (captures.isNotEmpty) {
      var best = captures.first;
      var bestScore = AiHeuristics.captureScore(best, state, player);
      for (final c in captures.skip(1)) {
        final s = AiHeuristics.captureScore(c, state, player);
        if (s > bestScore) {
          best = c;
          bestScore = s;
        }
      }
      return MoveIntent.place(best.point);
    }

    // (2) Escape atari: extend OR capture the attacker.
    final saves = AiHeuristics.savingMoves(state, legal: legal);
    if (saves.isNotEmpty) {
      return MoveIntent.place(_pickBest(saves, board, player, state));
    }

    // (3) Drop self-atari moves unless they capture (already handled above).
    final safe = legal
        .where((p) => !AiHeuristics.isSelfAtari(state, p))
        .toList(growable: false);
    final pool = safe.isEmpty ? legal : safe;

    // Heuristic pre-score, then 1-ply lookahead on the top N.
    final scored = pool
        .map((p) => (
              p,
              _scoreMove(
                p,
                board,
                player,
                opponent,
                state.moveNumber,
                state.lastMove?.point,
              )
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

  int _lookahead1Ply(GameState state, Point p) {
    final res = Rules.apply(state, MoveIntent.place(p));
    if (!res.isAccepted) return -1 << 31;
    final after = res.newStateAs<GameState>();
    var score = (res.move?.captured.length ?? 0) * 10;
    score += _scoreMove(p, state.board, state.currentPlayer,
        state.currentPlayer.other, state.moveNumber, state.lastMove?.point);

    final oppLegal = Rules.legalPlacements(after).take(40).toList(growable: false);
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
    return AiHeuristics.localMoveScore(
          p,
          board,
          player,
          opponent,
          moveNumber,
          lastOpponentMove,
        ) +
        _random.nextInt(2);
  }
}
