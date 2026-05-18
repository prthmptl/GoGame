import 'dart:math' as math;

import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/ai/advanced_ai.dart';
import 'package:go_game/src/domain/ai/beginner_ai.dart';
import 'package:go_game/src/domain/ai/intermediate_ai.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';

void main() {
  test('all AI levels open legally across supported board sizes', () {
    for (final size in const [9, 13, 19]) {
      final state = GameState.newGame(GameConfig(boardSize: size));
      final ais = [
        BeginnerAi(random: math.Random(1)),
        IntermediateAi(random: math.Random(1)),
        AdvancedAi(random: math.Random(1)),
      ];

      for (final ai in ais) {
        final move = ai.chooseMove(state);
        expect(
          Rules.apply(state, move).isAccepted,
          isTrue,
          reason: '${ai.runtimeType} should open legally on ${size}x$size',
        );
      }
    }
  });

  test('intermediate AI opens on the center of a 9x9 board', () {
    final state = GameState.newGame(const GameConfig(boardSize: 9));
    final move = IntermediateAi(random: math.Random(1)).chooseMove(state);
    expect(move.type, MoveType.placeStone);
    expect(move.point, const Point(4, 4));
  });

  test('advanced AI returns a legal move in a small fighting position', () {
    var state = GameState.newGame(const GameConfig(boardSize: 9));
    for (final p in const [
      Point(4, 4),
      Point(4, 5),
      Point(5, 4),
      Point(3, 4),
      Point(5, 5),
      Point(3, 5),
    ]) {
      final res = Rules.apply(state, MoveIntent.place(p));
      expect(res.isAccepted, isTrue);
      state = res.newStateAs<GameState>();
    }

    final move = AdvancedAi(random: math.Random(1)).chooseMove(state);
    final applied = Rules.apply(state, move);
    expect(applied.isAccepted, isTrue);
  });
}
