import '../domain/board.dart';
import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/rules.dart';
import 'sgf.dart';
import 'sgf_tree.dart';

class SgfImport {
  /// Replay only the first variation, respecting escaped text and move colours.
  static GameState import(String sgf) {
    final root = SgfTreeParser.parse(sgf);
    var state = initialState(root);
    SgfTreeNode? node = root;
    while (node != null) {
      state = applyNode(state, node, allowSetup: identical(node, root));
      node = node.children.isEmpty ? null : node.children.first;
    }
    return state;
  }

  static GameState initialState(SgfTreeNode root) {
    final size = int.tryParse(root.prop('SZ') ?? '19');
    final ruleset = Sgf.rulesetFromSgf(root.prop('RU') ?? 'Chinese');
    final defaults = RulesetDefaults.of(ruleset);
    final komi = double.tryParse(root.prop('KM') ?? '${defaults.komi}');
    final handicap = int.tryParse(root.prop('HA') ?? '0');
    if (size == null ||
        size < 2 ||
        size > 25 ||
        komi == null ||
        !komi.isFinite ||
        handicap == null ||
        handicap < 0 ||
        handicap > 9 ||
        (root.prop('GM') != null && root.prop('GM') != '1')) {
      throw const FormatException('Invalid Go SGF header');
    }
    var state = GameState.newGame(GameConfig(
        boardSize: size,
        ruleset: ruleset,
        komi: komi,
        handicap: handicap,
        allowSuicide: defaults.allowSuicide,
        superkoMode: defaults.superkoMode));
    if (root.properties.containsKey('AB') ||
        root.properties.containsKey('AW') ||
        root.properties.containsKey('AE')) {
      var board = Board.empty(size);
      for (final key in ['AB', 'AW', 'AE']) {
        for (final raw in root.properties[key] ?? <String>[]) {
          final bounds = raw.split(':');
          final first = point(bounds.first, size);
          final last = bounds.length == 1 ? first : point(bounds.last, size);
          if (bounds.length > 2 ||
              first.row > last.row ||
              first.col > last.col) {
            throw const FormatException('Invalid setup rectangle');
          }
          for (var row = first.row; row <= last.row; row++) {
            for (var col = first.col; col <= last.col; col++) {
              board = board.setCell(
                  Point(row, col),
                  key == 'AB'
                      ? CellState.black
                      : key == 'AW'
                          ? CellState.white
                          : CellState.empty);
            }
          }
        }
      }
      state = state.copyWith(board: board);
    }
    final pl = root.prop('PL');
    if (pl != null && pl != 'B' && pl != 'W') {
      throw const FormatException('Invalid player to move');
    }
    final player = pl == null
        ? state.currentPlayer
        : pl == 'B'
            ? StoneColor.black
            : StoneColor.white;
    return state.copyWith(currentPlayer: player, previousHashes: {
      Rules.positionHash(state.config.superkoMode, state.board, player),
    });
  }

  static GameState applyNode(GameState state, SgfTreeNode node,
      {bool allowSetup = false}) {
    if (!allowSetup &&
        ['AB', 'AW', 'AE', 'PL'].any(node.properties.containsKey)) {
      throw const FormatException(
          'Setup changes after the root are not supported');
    }
    final black = node.prop('B'), white = node.prop('W');
    if (black == null && white == null) return state;
    if (black != null && white != null) {
      throw const FormatException('Two moves in one SGF node');
    }
    final player = black != null ? StoneColor.black : StoneColor.white;
    if (player != state.currentPlayer) {
      throw const FormatException('Wrong player in SGF');
    }
    final raw = black ?? white!;
    final intent = raw.isEmpty || (raw == 'tt' && state.board.size <= 19)
        ? const MoveIntent.pass()
        : MoveIntent.place(point(raw, state.board.size));
    if (state.status == GameStatus.scoring) {
      state = state.copyWith(status: GameStatus.active, consecutivePasses: 0);
    }
    final applied = Rules.apply(state, intent);
    if (!applied.isAccepted) {
      throw FormatException('Illegal SGF move: ${applied.reason}');
    }
    return applied.newStateAs<GameState>();
  }

  static Point point(String raw, int size) {
    if (raw.length != 2) throw const FormatException('Invalid SGF coordinate');
    final col = raw.codeUnitAt(0) - 97, row = raw.codeUnitAt(1) - 97;
    if (row < 0 || row >= size || col < 0 || col >= size) {
      throw const FormatException('SGF coordinate outside board');
    }
    return Point(row, col);
  }
}
