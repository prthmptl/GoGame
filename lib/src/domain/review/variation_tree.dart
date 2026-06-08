import '../../sgf/sgf_tree.dart';
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

  int _nextId;

  VariationTree({
    required this.config,
    required this.root,
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
    var state = GameState.newGame(config.toGameConfig());
    for (final node in path) {
      final move = node.move;
      if (move == null) continue;
      if (move.player != state.currentPlayer) return null;
      final intent = switch (move.type) {
        MoveType.pass => const MoveIntent.pass(),
        MoveType.resign => const MoveIntent.resign(),
        MoveType.placeStone => MoveIntent.place(move.point!),
      };
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

  bool _walk(VariationNode current, VariationNode target,
      List<VariationNode> path) {
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
    if (a.type != b.type) return false;
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
    return VariationTree(config: config, root: root, nextId: idCounter);
  }

  static VariationTree build(SgfTreeNode root) {
    final config = _parseConfig(root);
    var idCounter = 0;
    final rootNode = VariationNode(id: idCounter++);
    // The SGF root contains header properties; its first child is the first
    // "real" node. Replay through the rules engine so that captures and move
    // numbers are attached correctly.
    final initialState = GameState.newGame(config.toGameConfig());
    _appendChildren(root.children, rootNode, initialState, () => idCounter++);
    return VariationTree(config: config, root: rootNode, nextId: idCounter);
  }

  static VariationConfig _parseConfig(SgfTreeNode root) {
    final size = int.tryParse(root.prop('SZ') ?? '') ?? 19;
    final komi = double.tryParse(root.prop('KM') ?? '') ?? 7.5;
    final handicap = int.tryParse(root.prop('HA') ?? '') ?? 0;
    final ruRaw = root.prop('RU');
    final ruleset = _parseRuleset(ruRaw);
    return VariationConfig(
      boardSize: size,
      ruleset: ruleset,
      komi: komi,
      handicap: handicap,
    );
  }

  static Ruleset _parseRuleset(String? raw) {
    if (raw == null) return Ruleset.chinese;
    final low = raw.toLowerCase();
    if (low.contains('japanese')) return Ruleset.japanese;
    if (low.contains('korean')) return Ruleset.korean;
    if (low.contains('aga')) return Ruleset.aga;
    if (low.contains('ing')) return Ruleset.ing;
    if (low.contains('new zealand') || low == 'nz') return Ruleset.newZealand;
    if (low.contains('tromp')) return Ruleset.trompTaylor;
    return Ruleset.chinese;
  }

  static void _appendChildren(
    List<SgfTreeNode> sgfChildren,
    VariationNode parent,
    GameState state,
    int Function() allocateId,
  ) {
    for (final sgf in sgfChildren) {
      final move = _parseMove(sgf, state);
      if (move == null) continue;
      if (move.player != state.currentPlayer) continue;
      final res = Rules.apply(
        state,
        switch (move.type) {
          MoveType.pass => const MoveIntent.pass(),
          MoveType.resign => const MoveIntent.resign(),
          MoveType.placeStone => MoveIntent.place(move.point!),
        },
      );
      if (!res.isAccepted) continue;
      final nextState = res.newStateAs<GameState>();
      // Use the move object the rules engine produced — that one carries the
      // correct move number + captured list.
      final node = VariationNode(
        id: allocateId(),
        move: res.move,
        comment: sgf.prop('C'),
      );
      parent.children.add(node);
      _appendChildren(sgf.children, node, nextState, allocateId);
    }
  }

  static Move? _parseMove(SgfTreeNode sgf, GameState state) {
    final boardSize = state.board.size;
    final bRaw = sgf.prop('B');
    final wRaw = sgf.prop('W');
    String? raw;
    StoneColor player;
    if (bRaw != null) {
      raw = bRaw;
      player = StoneColor.black;
    } else if (wRaw != null) {
      raw = wRaw;
      player = StoneColor.white;
    } else {
      return null;
    }
    final isPass = raw.isEmpty || raw == 'tt';
    final point = isPass ? null : _parsePoint(raw, boardSize);
    if (!isPass && point == null) return null;
    return Move(
      moveNumber: state.moveNumber + 1,
      player: player,
      type: isPass ? MoveType.pass : MoveType.placeStone,
      point: point,
      captured: const [],
    );
  }

  static Point? _parsePoint(String raw, int boardSize) {
    if (raw.length < 2) return null;
    final col = _decodeCoord(raw.codeUnitAt(0));
    final row = _decodeCoord(raw.codeUnitAt(1));
    if (col < 0 || col >= boardSize) return null;
    if (row < 0 || row >= boardSize) return null;
    return Point(row, col);
  }

  static int _decodeCoord(int code) {
    if (code >= 0x61 && code <= 0x7A) return code - 0x61;
    if (code >= 0x41 && code <= 0x5A) return code - 0x41;
    return -1;
  }
}
