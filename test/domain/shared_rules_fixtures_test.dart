import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/rules.dart';

void main() {
  final fixtures = jsonDecode(
          File('backend/internal/goban/testdata/rules.json').readAsStringSync())
      as List;
  for (final f in fixtures) {
    test('shared rules: ${f['name']}', () {
      final rules = Ruleset.values.firstWhere((r) =>
          r.name.toLowerCase() == (f['rules'] as String).replaceAll('_', ''));
      final defaults = RulesetDefaults.of(rules);
      var state = GameState.newGame(GameConfig(
          boardSize: f['size'] as int,
          ruleset: rules,
          komi: defaults.komi,
          handicap: f['handicap'] as int,
          allowSuicide: defaults.allowSuicide,
          superkoMode: defaults.superkoMode));
      for (final step in f['steps'] as List) {
        final intent = switch (step['kind']) {
          'pass' => const MoveIntent.pass(),
          'resign' => const MoveIntent.resign(),
          _ => MoveIntent.place(Point(step['row'] as int, step['col'] as int)),
        };
        final applied = Rules.apply(state, intent);
        expect(applied.isAccepted, step['accept'],
            reason: '$step: ${applied.reason}');
        if (applied.isAccepted) state = applied.newStateAs<GameState>();
      }
      final expected = <Point, CellState>{};
      for (final p in f['black'] as List) {
        expected[Point(p[0] as int, p[1] as int)] = CellState.black;
      }
      for (final p in f['white'] as List) {
        expected[Point(p[0] as int, p[1] as int)] = CellState.white;
      }
      for (var r = 0; r < state.board.size; r++) {
        for (var c = 0; c < state.board.size; c++) {
          final point = Point(r, c);
          expect(state.board.cellAt(point), expected[point] ?? CellState.empty,
              reason: '$point');
        }
      }
      expect(state.currentPlayer.name, f['toMove']);
      expect(state.status.name, f['status']);
      expect(state.moveNumber, f['moves']);
      expect([state.capturesByBlack, state.capturesByWhite], f['captures']);
    });
  }
}
