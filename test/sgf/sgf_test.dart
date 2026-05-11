import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';
import 'package:go_game/src/sgf/sgf.dart';

void main() {
  test('exports basic game', () {
    var s = GameState.newGame(const GameConfig(boardSize: 9, komi: 7.5));
    s = Rules.apply(s, const MoveIntent.place(Point(2, 3)))
        .newStateAs<GameState>();
    s = Rules.apply(s, const MoveIntent.pass()).newStateAs<GameState>();
    final sgf = Sgf.export(
      s,
      blackName: 'B',
      whiteName: 'W',
      date: DateTime(2026, 5, 3),
    );
    expect(sgf.startsWith('(;GM[1]FF[4]'), isTrue);
    expect(sgf.contains('SZ[9]'), isTrue);
    expect(sgf.contains('KM[7.5]'), isTrue);
    expect(sgf.contains('RU[Chinese]'), isTrue);
    expect(sgf.contains('GC[Chinese rules'), isTrue);
    expect(sgf.contains(';B[dc]'), isTrue); // (row=2,col=3) -> col 'd', row 'c'
    expect(sgf.contains(';W[]'), isTrue);
    expect(sgf.endsWith(')'), isTrue);
  });

  test('emits matching RU and GC for Japanese rules', () {
    final s = GameState.newGame(const GameConfig(
      boardSize: 9,
      ruleset: Ruleset.japanese,
      komi: 6.5,
      superkoMode: SuperkoMode.none,
    ));
    final sgf = Sgf.export(s, date: DateTime(2026, 5, 11));
    expect(sgf.contains('RU[Japanese]'), isTrue);
    expect(sgf.contains('GC[Japanese rules'), isTrue);
    expect(sgf.contains('territory + prisoners'), isTrue);
    expect(sgf.contains('basic ko'), isTrue);
  });
}
