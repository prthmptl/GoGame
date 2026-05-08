import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';

void main() {
  GameState newGame({int size = 9, double komi = 7.5}) =>
      GameState.newGame(GameConfig(boardSize: size, komi: komi));

  GameState play(GameState state, int r, int c) {
    final res = Rules.apply(state, MoveIntent.place(Point(r, c)));
    expect(res.isAccepted, isTrue, reason: 'expected accepted at ($r,$c)');
    return res.newStateAs<GameState>();
  }

  GameState pass(GameState state) {
    final res = Rules.apply(state, const MoveIntent.pass());
    return res.newStateAs<GameState>();
  }

  test('basic placement', () {
    var s = newGame();
    s = play(s, 4, 4);
    expect(s.board.cellAt(const Point(4, 4)), CellState.black);
    expect(s.currentPlayer, StoneColor.white);
  });

  test('reject occupied', () {
    var s = newGame();
    s = play(s, 4, 4);
    final res = Rules.apply(s, const MoveIntent.place(Point(4, 4)));
    expect(res.isAccepted, isFalse);
    expect(res.reason, MoveRejection.occupied);
  });

  test('reject out of bounds', () {
    final s = newGame(size: 9);
    final res = Rules.apply(s, const MoveIntent.place(Point(9, 0)));
    expect(res.isAccepted, isFalse);
    expect(res.reason, MoveRejection.outOfBounds);
  });

  test('single stone capture', () {
    var s = newGame();
    s = play(s, 0, 1); // B
    s = play(s, 0, 0); // W (corner)
    s = play(s, 1, 0); // B captures W at (0,0)
    expect(s.board.cellAt(const Point(0, 0)), CellState.empty);
    expect(s.capturesByBlack, 1);
  });

  test('reject suicide', () {
    var s = newGame();
    s = play(s, 5, 5); // B (away)
    s = play(s, 0, 1); // W
    s = play(s, 5, 6); // B
    s = play(s, 1, 0); // W
    final res = Rules.apply(s, const MoveIntent.place(Point(0, 0)));
    expect(res.isAccepted, isFalse);
    expect(res.reason, MoveRejection.suicide);
  });

  test('capture beats apparent suicide', () {
    var s = newGame();
    s = play(s, 1, 0); // B
    s = play(s, 0, 0); // W
    s = play(s, 1, 1); // B
    s = play(s, 0, 1); // W
    s = play(s, 1, 2); // B
    s = play(s, 8, 8); // W
    s = play(s, 0, 2); // B captures white group
    expect(s.board.cellAt(const Point(0, 0)), CellState.empty);
    expect(s.board.cellAt(const Point(0, 1)), CellState.empty);
    expect(s.capturesByBlack >= 2, isTrue);
  });

  test('simple ko blocks immediate recapture', () {
    var s = newGame();
    s = play(s, 2, 2); // B
    s = play(s, 2, 3); // W
    s = play(s, 3, 1); // B
    s = play(s, 3, 4); // W
    s = play(s, 4, 2); // B
    s = play(s, 4, 3); // W
    s = play(s, 3, 3); // B
    s = play(s, 3, 2); // W captures B at (3,3)
    final res = Rules.apply(s, const MoveIntent.place(Point(3, 3)));
    expect(res.isAccepted, isFalse);
    expect(res.reason, MoveRejection.koViolation);
  });

  test('double pass enters scoring', () {
    var s = newGame();
    s = pass(s);
    s = pass(s);
    expect(s.status, GameStatus.scoring);
  });

  test('resign ends game', () {
    final s = newGame();
    final res = Rules.apply(s, const MoveIntent.resign());
    expect(res.isAccepted, isTrue);
    expect(res.newStateAs<GameState>().status, GameStatus.resigned);
  });

  test('handicap places black stones', () {
    final s = GameState.newGame(const GameConfig(boardSize: 9, handicap: 2));
    var blacks = 0;
    for (var r = 0; r < 9; r++) {
      for (var c = 0; c < 9; c++) {
        if (s.board.cellAt(Point(r, c)) == CellState.black) blacks++;
      }
    }
    expect(blacks, 2);
    expect(s.currentPlayer, StoneColor.white);
  });
}
