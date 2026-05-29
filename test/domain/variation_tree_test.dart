import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/review/variation_tree.dart';
import 'package:go_game/src/sgf/sgf_tree.dart';

void main() {
  group('SgfTreeParser', () {
    test('parses a simple main line', () {
      final root =
          SgfTreeParser.parse('(;FF[4]SZ[9];B[ee];W[dd])');
      expect(root.prop('SZ'), '9');
      expect(root.children, hasLength(1));
      final b = root.children.first;
      expect(b.prop('B'), 'ee');
      expect(b.children, hasLength(1));
      expect(b.children.first.prop('W'), 'dd');
    });

    test('parses variations as siblings under the divergence point', () {
      final root = SgfTreeParser.parse(
          '(;SZ[9];B[ee](;W[dd];B[cc])(;W[ee])(;W[fe]))');
      // Root → B[ee] → 3 children (the variations).
      final move1 = root.children.first;
      expect(move1.prop('B'), 'ee');
      expect(move1.children, hasLength(3));
      expect(move1.children[0].prop('W'), 'dd');
      expect(move1.children[1].prop('W'), 'ee');
      expect(move1.children[2].prop('W'), 'fe');
    });

    test('handles escaped values in comments', () {
      final root =
          SgfTreeParser.parse(r'(;SZ[9];B[ee]C[hello \] world])');
      expect(root.children.first.prop('C'), 'hello ] world');
    });
  });

  group('VariationTreeBuilder', () {
    test('builds typed nodes from SGF and links the main line', () {
      final sgf = SgfTreeParser.parse('(;SZ[9];B[ee];W[dd];B[ge])');
      final tree = VariationTreeBuilder.build(sgf);
      expect(tree.config.boardSize, 9);
      expect(tree.root.move, isNull);
      var node = tree.root;
      for (final expected in const [
        (StoneColor.black, Point(4, 4)),
        (StoneColor.white, Point(3, 3)),
        (StoneColor.black, Point(4, 6)),
      ]) {
        expect(node.children, isNotEmpty);
        node = node.children.first;
        expect(node.move!.player, expected.$1);
        expect(node.move!.point, expected.$2);
      }
    });

    test('keeps each SGF variation as a child node', () {
      final sgf =
          SgfTreeParser.parse('(;SZ[9];B[ee](;W[dd])(;W[ge]))');
      final tree = VariationTreeBuilder.build(sgf);
      final blackMove = tree.root.children.first;
      expect(blackMove.children, hasLength(2));
      expect(blackMove.children[0].move!.point, const Point(3, 3));
      expect(blackMove.children[1].move!.point, const Point(4, 6));
    });
  });

  group('VariationTree navigation', () {
    test('buildStateTo replays the move list correctly', () {
      final sgf = SgfTreeParser.parse('(;SZ[9];B[ee];W[dd])');
      final tree = VariationTreeBuilder.build(sgf);
      final terminal = tree.root.children.first.children.first;
      final state = tree.buildStateTo(terminal)!;
      expect(state.board.cellAt(const Point(4, 4)), CellState.black);
      expect(state.board.cellAt(const Point(3, 3)), CellState.white);
      expect(state.history, hasLength(2));
    });

    test('addExploration appends a child and returns existing matches', () {
      final sgf = SgfTreeParser.parse('(;SZ[9];B[ee])');
      final tree = VariationTreeBuilder.build(sgf);
      final blackMove = tree.root.children.first;
      const move = Move(
        moveNumber: 2,
        player: StoneColor.white,
        type: MoveType.placeStone,
        point: Point(3, 3),
        captured: [],
      );
      final first = tree.addExploration(blackMove, move);
      final second = tree.addExploration(blackMove, move);
      expect(identical(first, second), isTrue);
      expect(blackMove.children, hasLength(1));
      expect(first.isExploration, isTrue);
    });
  });
}
