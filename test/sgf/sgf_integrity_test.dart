import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/review/variation_tree.dart';
import 'package:go_game/src/sgf/sgf.dart';
import 'package:go_game/src/sgf/sgf_import.dart';
import 'package:go_game/src/sgf/sgf_tree.dart';

void main() {
  test('main line ignores sibling variations and move-like text in comments',
      () {
    final state = SgfImport.import(
        '(;SZ[9]C[example ;B[dd\\] text];B[aa](;W[bb])(;W[cc]))');
    expect(state.history.length, 2);
    expect(state.board.cellAt(const Point(1, 1)), CellState.white);
    expect(state.board.cellAt(const Point(2, 2)), CellState.empty);
  });
  test('setup and annotation-only nodes survive import and review', () {
    const sgf = '(;SZ[9]AB[aa][ab]AW[cc]PL[W];C[teaching];W[dd])';
    final state = SgfImport.import(sgf);
    final tree = VariationTreeBuilder.build(SgfTreeParser.parse(sgf));
    final reviewed = tree.buildStateTo(tree.root.children.single)!;
    expect(state.board.cellAt(const Point(0, 0)), CellState.black);
    expect(reviewed.board.cellAt(const Point(1, 0)), CellState.black);
    expect(reviewed.board.cellAt(const Point(3, 3)), CellState.white);
  });
  test(
      'handicap export includes explicit setup stones and escapes player names',
      () {
    final state =
        GameState.newGame(const GameConfig(boardSize: 9, handicap: 2));
    final sgf = Sgf.export(state, blackName: r'A]B\C');
    final root = SgfTreeParser.parse(sgf);
    expect(root.properties['AB']?.length, 2);
    expect(root.prop('PB'), r'A]B\C');
    expect(SgfImport.import(sgf).currentPlayer, StoneColor.white);
  });
  test('invalid inputs reject instead of returning partial games', () {
    for (final text in [
      'not sgf',
      '(;SZ[999999])',
      '(;SZ[9];W[aa])',
      '(;SZ[9];B[zz])',
      '(;SZ[9];B[aa];W[aa])',
      '(;SZ[9];B[aa];AB[bb];W[cc])'
    ]) {
      expect(() => SgfImport.import(text), throwsA(isA<Exception>()));
    }
    expect(() => SgfTreeParser.parse('(;SZ[9]) trailing'),
        throwsA(isA<Exception>()));
  });
  test('tt is a placement on larger boards and root moves are preserved', () {
    final state = SgfImport.import('(;SZ[25]B[tt])');
    expect(state.board.cellAt(const Point(19, 19)), CellState.black);
  });
}
