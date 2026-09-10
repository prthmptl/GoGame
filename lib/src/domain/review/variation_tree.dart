import '../../sgf/sgf_tree.dart';
import '../../sgf/sgf_import.dart';
import '../game_state.dart';
import '../models.dart';
import '../rules.dart';

class VariationConfig {
  final int boardSize;
  final Ruleset ruleset;
  final double komi;
  final int handicap;

  const VariationConfig({
    this.boardSize = 19,
    this.ruleset = Ruleset.chinese,
    this.komi = 7.5,
    this.handicap = 0,
  });

  GameConfig toGameConfig() {
    final defaults = RulesetDefaults.of(ruleset);
    return GameConfig(
      boardSize: boardSize,
      ruleset: ruleset,
      komi: komi,
      handicap: handicap,
      allowSuicide: defaults.allowSuicide,
      superkoMode: defaults.superkoMode,
    );
  }
}

class VariationNode {
  /// Stable identifier within a tree (assigned at parse time).
  final int id;

  /// The move that produces this node, or `null` for the implicit root before
  /// any moves have been played.
  final Move? move;

  /// Optional comment / annotation for display.
  final String? comment;

  /// Children — the first child is the canonical "main line" continuation.
  final List<VariationNode> children;

  /// True if this node was inserted at runtime by the user exploring a
  /// hypothetical move (rather than loaded from SGF).
  final bool isExploration;

  VariationNode({
    required this.id,
    this.move,
    this.comment,
    List<VariationNode>? children,
    this.isExploration = false,
  }) : children = children ?? <VariationNode>[];

  bool get isRoot => move == null;
}

class VariationTree {
  final VariationConfig config;
  final VariationNode root;
  final GameState? initialState;

  int _nextId;

  VariationTree({
    required this.config,
    required this.root,
    this.initialState,
    required int nextId,
  }) : _nextId = nextId;

  /// Allocate a fresh node id, used when the user tries a new move at runtime.
  int allocateId() => _nextId++;

  /// Replay all moves from [root] down to [target], building the resulting
  /// [GameState]. Returns `null` if the path contains an illegal move
  /// (e.g. a now-invalid SGF position).
  GameState? buildStateTo(VariationNode target) {
    final path = pathTo(target);
    if (path == null) return null;
    var state = initialState ?? GameState.newGame(config.toGameConfig());
    for (final node in path) {
      final move = node.move;
      if (move == null) continue;
      if (move.player != state.currentPlayer) return null;
      final intent = switch (move.type) {
        MoveType.pass => const MoveIntent.pass(),
        MoveType.resign => const MoveIntent.resign(),
        MoveType.placeStone => MoveIntent.place(move.point!),
      };
      if (state.status == GameStatus.scoring) {
        state = state.copyWith(status: GameStatus.active, consecutivePasses: 0);
      }
      final r = Rules.apply(state, intent);
      if (!r.isAccepted) return null;
      state = r.newStateAs<GameState>();
    }
    return state;
  }

  /// Find the path from [root] to [target] inclusive.
  List<VariationNode>? pathTo(VariationNode target) {
    final out = <VariationNode>[];
    if (_walk(root, target, out)) return out;
    return null;
  }

  bool _walk(
      VariationNode current, VariationNode target, List<VariationNode> path) {
    path.add(current);
    if (identical(current, target)) return true;
    for (final c in current.children) {
      if (_walk(c, target, path)) return true;
    }
    path.removeLast();
    return false;
  }

  /// Adds an exploration child to [parent] with the given move; returns the
  /// new node. If a child with the same move already exists it is returned
  /// instead of being duplicated.
  VariationNode addExploration(VariationNode parent, Move move) {
    for (final c in parent.children) {
      if (_movesMatch(c.move, move)) return c;
    }
    final node = VariationNode(
      id: allocateId(),
      move: move,
      isExploration: true,
    );
    parent.children.add(node);
    return node;
  }

  static bool _movesMatch(Move? a, Move b) {
    if (a == null) return false;
    if (a.type != b.type || a.player != b.player) return false;
    if (a.type != MoveType.placeStone) return true;
    return a.point == b.point;
  }
}

/// Build a [VariationTree] from a parsed SGF root.
class VariationTreeBuilder {
  /// Construct a linear tree from a completed game's [history]. Used when the
  /// review screen opens a saved game with no SGF variations.
  static VariationTree fromHistory(GameState game) {
    final config = VariationConfig(
      boardSize: game.config.boardSize,
      ruleset: game.config.ruleset,
      komi: game.config.komi,
      handicap: game.config.handicap,
    );
    var idCounter = 0;
    final root = VariationNode(id: idCounter++);
    var parent = root;
    for (final move in game.history) {
      final node = VariationNode(id: idCounter++, move: move);
      parent.children.add(node);
      parent = node;
    }
    return VariationTree(
        config: config,
        root: root,
        nextId: idCounter,
        initialState: GameState.newGame(game.config));
  }

  static VariationTree build(SgfTreeNode root) {
    final initialState = SgfImport.initialState(root);
    final cfg = initialState.config;
    final config = VariationConfig(
        boardSize: cfg.boardSize,
        ruleset: cfg.ruleset,
        komi: cfg.komi,
        handicap: cfg.handicap);
    var idCounter = 0;
    final rootNode = VariationNode(id: idCounter++, comment: root.prop('C'));
    _appendChildren([root], rootNode, initialState, () => idCounter++,
        allowSetup: true);
    return VariationTree(
        config: config,
        root: rootNode,
        nextId: idCounter,
        initialState: initialState);
  }

  static void _appendChildren(
    List<SgfTreeNode> sgfChildren,
    VariationNode parent,
    GameState state,
    int Function() allocateId, {
    bool allowSetup = false,
  }) {
    for (final sgf in sgfChildren) {
      final nextState = SgfImport.applyNode(state, sgf, allowSetup: allowSetup);
      if (identical(nextState, state)) {
        _appendChildren(sgf.children, parent, state, allocateId);
        continue;
      }
      final node = VariationNode(
          id: allocateId(), move: nextState.lastMove, comment: sgf.prop('C'));
      parent.children.add(node);
      _appendChildren(sgf.children, node, nextState, allocateId);
    }
  }
}
