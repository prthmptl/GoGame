import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';

void main() {
  test('first capture immediately ends an Atari Go game', () {
    var state = GameState.newGame(const GameConfig(
      boardSize: 9,
      variant: GameVariant.atariGo,
    ));
    // Set up white in atari at (3,3) by playing the surrounding stones.
    for (final p in const [
      Point(2, 3), // black dc
      Point(3, 3), // white dd (gets surrounded)
      Point(3, 2), // black cd
      Point(0, 0), // white tengen far away
      Point(3, 4), // black ed
      Point(0, 1), // white passes time
    ]) {
      final res = Rules.apply(state, MoveIntent.place(p));
      expect(res.isAccepted, isTrue);
      state = res.newStateAs<GameState>();
    }
    expect(state.status, GameStatus.active);
    final capture = Rules.apply(state, const MoveIntent.place(Point(4, 3)));
    expect(capture.isAccepted, isTrue);
    final next = capture.newStateAs<GameState>();
    expect(next.status, GameStatus.completed);
  });

  test('standard variant does not end on a capture', () {
    var state = GameState.newGame(const GameConfig(boardSize: 9));
    for (final p in const [
      Point(2, 3),
      Point(3, 3),
      Point(3, 2),
      Point(0, 0),
      Point(3, 4),
      Point(0, 1),
    ]) {
      final res = Rules.apply(state, MoveIntent.place(p));
      state = res.newStateAs<GameState>();
    }
    final capture = Rules.apply(state, const MoveIntent.place(Point(4, 3)));
    final next = capture.newStateAs<GameState>();
    expect(next.status, GameStatus.active);
  });
}
