import 'dart:async';

import '../ai/ai_heuristics.dart';
import '../ai/beginner_ai.dart';
import '../ai/go_ai.dart';
import '../ai/intermediate_ai.dart';
import '../game_state.dart';
import '../models.dart';
import '../rules.dart';
import '../scoring.dart';

enum MoveQuality { best, good, inaccuracy, mistake, blunder, notApplicable }

extension MoveQualityLabel on MoveQuality {
  String get label => switch (this) {
        MoveQuality.best => 'Best',
        MoveQuality.good => 'Good',
        MoveQuality.inaccuracy => 'Inaccuracy',
        MoveQuality.mistake => 'Mistake',
        MoveQuality.blunder => 'Blunder',
        MoveQuality.notApplicable => '—',
      };
}

class MoveAnalysis {
  final int moveNumber;
  final StoneColor player;
  final MoveQuality quality;

  /// Heuristic point loss vs. the AI's preferred move (from [player]'s
  /// perspective). 0 means the chosen move matched the AI; higher is worse.
  final int pointsLost;

  /// AI's recommended move at this position, or `null` for pass/resign moves.
  final Point? recommended;

  /// Naive territory estimate after the actual move, from black's perspective
  /// (positive = black ahead). Drives the trend graph.
  final double blackLead;

  /// Comment for display ("you missed the capture", "AI preferred E5", etc.).
  final String? comment;

  const MoveAnalysis({
    required this.moveNumber,
    required this.player,
    required this.quality,
    required this.pointsLost,
    required this.recommended,
    required this.blackLead,
    required this.comment,
  });
}

class ReviewReport {
  final List<MoveAnalysis> moves;
  final int blunders;
  final int mistakes;
  final int inaccuracies;

  const ReviewReport({
    required this.moves,
    required this.blunders,
    required this.mistakes,
    required this.inaccuracies,
  });
}

enum ReviewDepth { quick, standard }

class ReviewAnalyzer {
  final ReviewDepth depth;

  ReviewAnalyzer({this.depth = ReviewDepth.quick});

  /// Walk the game's move history and produce a per-move analysis report.
  ///
  /// The work is split across microtasks so the UI can stay responsive while
  /// long games (200+ moves) are analyzed.
  Future<ReviewReport> analyze(GameState game) async {
    final ai = _buildAi();
    final moves = <MoveAnalysis>[];
    var state = GameState.newGame(game.config);
    var blunders = 0;
    var mistakes = 0;
    var inaccuracies = 0;
    for (final move in game.history) {
      final res = Rules.apply(
        state,
        switch (move.type) {
          MoveType.pass => const MoveIntent.pass(),
          MoveType.resign => const MoveIntent.resign(),
          MoveType.placeStone => MoveIntent.place(move.point!),
        },
      );
      if (!res.isAccepted) break;
      final after = res.newStateAs<GameState>();

      MoveQuality quality = MoveQuality.notApplicable;
      int pointsLost = 0;
      Point? recommended;
      String? comment;
      if (move.type == MoveType.placeStone) {
        recommended = _safeRecommend(ai, state);
        if (recommended != null) {
          final actualScore = AiHeuristics.evaluate(after, move.player);
          final altApply =
              Rules.apply(state, MoveIntent.place(recommended));
          if (altApply.isAccepted) {
            final altState = altApply.newStateAs<GameState>();
            final altScore = AiHeuristics.evaluate(altState, move.player);
            pointsLost = (altScore - actualScore).clamp(0, 1 << 30).toInt();
            quality = _classify(pointsLost, sameAsAi: recommended == move.point);
            if (quality == MoveQuality.best) recommended = null;
          } else {
            quality = MoveQuality.good;
          }
        }
        if (quality == MoveQuality.blunder) blunders++;
        if (quality == MoveQuality.mistake) mistakes++;
        if (quality == MoveQuality.inaccuracy) inaccuracies++;
        if (recommended != null && quality != MoveQuality.best) {
          comment = 'AI preferred ${_coord(recommended, after.board.size)}.';
        }
      }

      final lead = _blackLead(after);
      moves.add(MoveAnalysis(
        moveNumber: move.moveNumber,
        player: move.player,
        quality: quality,
        pointsLost: pointsLost,
        recommended: recommended,
        blackLead: lead,
        comment: comment,
      ));
      state = after;
      // Yield to the event loop every few moves so the UI stays responsive.
      if (moves.length % 8 == 0) {
        await Future<void>.delayed(Duration.zero);
      }
    }
    return ReviewReport(
      moves: moves,
      blunders: blunders,
      mistakes: mistakes,
      inaccuracies: inaccuracies,
    );
  }

  GoAi _buildAi() => switch (depth) {
        ReviewDepth.quick => BeginnerAi(),
        ReviewDepth.standard => IntermediateAi(),
      };

  Point? _safeRecommend(GoAi ai, GameState state) {
    final intent = ai.chooseMove(state);
    if (intent.type != MoveType.placeStone) return null;
    return intent.point;
  }

  MoveQuality _classify(int pointsLost, {required bool sameAsAi}) {
    if (sameAsAi) return MoveQuality.best;
    if (pointsLost <= 4) return MoveQuality.good;
    if (pointsLost <= 12) return MoveQuality.inaccuracy;
    if (pointsLost <= 28) return MoveQuality.mistake;
    return MoveQuality.blunder;
  }

  double _blackLead(GameState state) {
    final score = Scoring.score(state);
    return score.blackTotal - score.whiteTotal;
  }

  /// Convert a board point to GTP-style coordinates ("D4", "Q16").
  static String _coord(Point p, int size) {
    const skipI = 8; // 'I' is skipped in Go coordinates
    final col = p.col;
    final letter = String.fromCharCode(
        0x41 + (col < skipI ? col : col + 1));
    final row = size - p.row;
    return '$letter$row';
  }
}
