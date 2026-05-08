import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';
import 'package:go_game/src/domain/scoring.dart';

void main() {
  test('empty board komi only', () {
    final s = GameState.newGame(const GameConfig(boardSize: 9, komi: 7.5));
    final res = Scoring.score(s);
    expect(res.blackArea, 0);
    expect(res.whiteTotal, closeTo(7.5, 1e-9));
    expect(res.resultString.startsWith('W+'), isTrue);
  });

  test('black wall territory', () {
    var s = GameState.newGame(const GameConfig(boardSize: 9, komi: 0.0));
    const blackPts = [
      Point(3, 0),
      Point(3, 1),
      Point(3, 2),
      Point(3, 3),
      Point(0, 3),
      Point(1, 3),
      Point(2, 3),
    ];
    const whitePts = [
      Point(8, 8),
      Point(8, 7),
      Point(7, 8),
      Point(7, 7),
      Point(8, 6),
      Point(6, 8),
      Point(6, 7),
    ];
    var bi = 0;
    var wi = 0;
    while (bi < blackPts.length || wi < whitePts.length) {
      if (bi < blackPts.length) {
        final r = Rules.apply(s, MoveIntent.place(blackPts[bi]));
        s = r.newStateAs<GameState>();
        bi++;
      }
      if (wi < whitePts.length) {
        final r = Rules.apply(s, MoveIntent.place(whitePts[wi]));
        s = r.newStateAs<GameState>();
        wi++;
      }
    }
    final score = Scoring.score(s);
    expect(score.blackTerritory, 9);
    expect(score.blackStones, 7);
    expect(score.whiteStones, 7);
    expect(score.blackArea, 16);
    expect(score.whiteTotal, closeTo(7.0, 1e-9));
  });

  test('dead stones are removed', () {
    var s = GameState.newGame(const GameConfig(boardSize: 9, komi: 0.0));
    s = Rules.apply(s, const MoveIntent.place(Point(4, 4)))
        .newStateAs<GameState>();
    final score = Scoring.score(s, deadStones: {const Point(4, 4)});
    expect(score.blackStones, 0);
  });
}
