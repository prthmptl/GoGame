import 'dart:math' as math;

import '../game_state.dart';
import '../models.dart';
import '../rules.dart';
import 'ai_heuristics.dart';
import 'go_ai.dart';

/// A stronger local engine: tactical priorities plus one opponent reply.
class IntermediateAi implements GoAi {
  final math.Random _random;

  IntermediateAi({math.Random? random}) : _random = random ?? math.Random();

  @override
  MoveIntent chooseMove(GameState state) {
    if (state.history.isEmpty) {
      final sp = AiHeuristics.starPoint(state.board.size);
      if (sp != null &&
          Rules.apply(state, MoveIntent.place(sp)).isAccepted) {
        return MoveIntent.place(sp);
      }
    }

    final legal = AiHeuristics.legalNonEyeMoves(state);
    if (legal.isEmpty) return const MoveIntent.pass();

    final captures = AiHeuristics.capturingMoves(state, legal: legal);
    if (captures.isNotEmpty) {
      return MoveIntent.place(_bestCapture(captures, state));
    }

    final saves = AiHeuristics.savingMoves(state, legal: legal);
    final pool = saves.isNotEmpty
        ? saves
        : AiHeuristics.candidateMoves(
            state,
            limit: _candidateLimit(state.board.size),
          );
    return MoveIntent.place(_pickBestBySearch(state, pool));
  }

  Point _bestCapture(List<AppliedMove> moves, GameState state) {
    final player = state.currentPlayer;
    var best = moves.first;
    var bestScore = AiHeuristics.captureScore(best, state, player);
    for (final move in moves.skip(1)) {
      final score = AiHeuristics.captureScore(move, state, player);
      if (score > bestScore) {
        best = move;
        bestScore = score;
      }
    }
    return best.point;
  }

  Point _pickBestBySearch(GameState state, List<Point> pool) {
    final player = state.currentPlayer;
    final width = _replyWidth(state.board.size);
    final scored = <({Point point, int score})>[];
    for (final p in pool) {
      final res = Rules.apply(state, MoveIntent.place(p));
      if (!res.isAccepted) continue;
      final next = res.newStateAs<GameState>();
      final score = AiSearch.minimax(
        next,
        player,
        depth: 1,
        width: width,
      );
      scored.add((point: p, score: score));
    }
    if (scored.isEmpty) return pool.first;

    scored.sort((a, b) => b.score.compareTo(a.score));
    final best = scored.first.score;
    final finalists = scored
        .where((e) => e.score >= best - 3)
        .map((e) => e.point)
        .toList(growable: false);
    return finalists[_random.nextInt(finalists.length)];
  }

  int _candidateLimit(int size) {
    if (size <= 9) return 18;
    if (size <= 13) return 20;
    return 24;
  }

  int _replyWidth(int size) {
    if (size <= 9) return 12;
    if (size <= 13) return 10;
    return 8;
  }
}
