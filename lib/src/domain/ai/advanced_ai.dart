import 'dart:math' as math;

import '../game_state.dart';
import '../models.dart';
import '../rules.dart';
import 'ai_heuristics.dart';
import 'go_ai.dart';

/// Stronger local engine with a two-stage search:
///   1. rank a modest pool with a cheap one-reply search
///   2. spend deeper reading only on the best few moves
///
/// This is still a handcrafted engine, not a modern neural Go engine. It should
/// feel materially stronger than [IntermediateAi] on local fights while staying
/// light enough for on-device play.
class AdvancedAi implements GoAi {
  final math.Random _random;

  AdvancedAi({math.Random? random}) : _random = random ?? math.Random();

  @override
  MoveIntent chooseMove(GameState state) {
    if (state.history.isEmpty) {
      final sp = AiHeuristics.starPoint(state.board.size);
      if (sp != null && Rules.apply(state, MoveIntent.place(sp)).isAccepted) {
        return MoveIntent.place(sp);
      }
    }

    final legal = AiHeuristics.legalNonEyeMoves(state);
    if (legal.isEmpty) return const MoveIntent.pass();

    final tactical = <Point>{
      ...AiHeuristics.capturingMoves(state, legal: legal).map((m) => m.point),
      ...AiHeuristics.savingMoves(state, legal: legal),
    };
    final broad = AiHeuristics.candidateMoves(
      state,
      limit: _candidateLimit(state.board.size),
    );
    final pool = <Point>{...tactical, ...broad}.toList(growable: false);

    final player = state.currentPlayer;
    final firstPass = _scoreMoves(
      state,
      pool,
      player,
      depth: 1,
      width: _firstPassWidth(state.board.size),
    );
    if (firstPass.isEmpty) return MoveIntent.place(pool.first);

    final scored = _scoreMoves(
      state,
      firstPass
          .take(_deepCandidateLimit(state.board.size))
          .map((e) => e.point)
          .toList(growable: false),
      player,
      depth: 2,
      width: _deepSearchWidth(state.board.size),
    );

    final ranked = scored.isEmpty ? firstPass : scored;
    final best = ranked.first.score;
    final finalists = ranked
        .where((e) => e.score >= best - 2)
        .map((e) => e.point)
        .toList(growable: false);
    return MoveIntent.place(finalists[_random.nextInt(finalists.length)]);
  }

  List<({Point point, int score})> _scoreMoves(
    GameState state,
    List<Point> moves,
    StoneColor player, {
    required int depth,
    required int width,
  }) {
    final scored = <({Point point, int score})>[];
    for (final p in moves) {
      final res = Rules.apply(state, MoveIntent.place(p));
      if (!res.isAccepted) continue;
      final next = res.newStateAs<GameState>();
      final immediateCaptureBonus = (res.move?.captured.length ?? 0) * 8;
      final score = immediateCaptureBonus +
          AiSearch.minimax(
            next,
            player,
            depth: depth,
            width: width,
          );
      scored.add((point: p, score: score));
    }
    scored.sort((a, b) => b.score.compareTo(a.score));
    return scored;
  }

  int _candidateLimit(int size) {
    if (size <= 9) return 14;
    if (size <= 13) return 14;
    return 12;
  }

  int _deepCandidateLimit(int size) {
    if (size <= 9) return 5;
    if (size <= 13) return 4;
    return 3;
  }

  int _firstPassWidth(int size) {
    if (size <= 9) return 8;
    if (size <= 13) return 8;
    return 6;
  }

  int _deepSearchWidth(int size) {
    if (size <= 9) return 6;
    if (size <= 13) return 5;
    return 4;
  }
}
