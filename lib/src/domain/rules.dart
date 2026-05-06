import 'game_state.dart';
import 'groups.dart';
import 'models.dart';

class Rules {
  static MoveResult apply(GameState state, MoveIntent intent) {
    if (state.status != GameStatus.active) {
      return MoveResult.rejected(MoveRejection.gameNotActive);
    }
    switch (intent.type) {
      case MoveType.pass:
        return _applyPass(state);
      case MoveType.resign:
        return _applyResign(state);
      case MoveType.placeStone:
        return _applyPlace(state, intent.point!);
    }
  }

  static MoveResult _applyPass(GameState state) {
    final passes = state.consecutivePasses + 1;
    final move = Move(
      moveNumber: state.moveNumber + 1,
      player: state.currentPlayer,
      type: MoveType.pass,
      point: null,
      captured: const [],
    );
    final nextStatus = passes >= 2 ? GameStatus.scoring : GameStatus.active;
    final next = state.copyWith(
      currentPlayer: state.currentPlayer.other,
      moveNumber: state.moveNumber + 1,
      koPoint: null,
      consecutivePasses: passes,
      status: nextStatus,
      lastMove: move,
      history: [...state.history, move],
    );
    return MoveResult.accepted(next, move);
  }

  static MoveResult _applyResign(GameState state) {
    final move = Move(
      moveNumber: state.moveNumber + 1,
      player: state.currentPlayer,
      type: MoveType.resign,
      point: null,
      captured: const [],
    );
    final next = state.copyWith(
      status: GameStatus.resigned,
      lastMove: move,
      history: [...state.history, move],
    );
    return MoveResult.accepted(next, move);
  }

  static MoveResult _applyPlace(GameState state, Point point) {
    final board = state.board;
    if (!board.inBounds(point)) {
      return MoveResult.rejected(MoveRejection.outOfBounds);
    }
    if (board.cellAt(point) != CellState.empty) {
      return MoveResult.rejected(MoveRejection.occupied);
    }
    if (state.koPoint == point) {
      return MoveResult.rejected(MoveRejection.koViolation);
    }

    final player = state.currentPlayer;
    final opponent = player.other;
    final placed = board.setCell(point, CellState.of(player));

    // Capture opponent groups touching the placed stone with no liberties.
    final captured = <Point>{};
    final seen = <Point>{};
    for (final n in placed.neighbors(point)) {
      if (placed.cellAt(n) == CellState.of(opponent) && !seen.contains(n)) {
        final group = findGroup(placed, n);
        seen.addAll(group.stones);
        if (group.liberties.isEmpty) captured.addAll(group.stones);
      }
    }
    final afterCapture = captured.isEmpty
        ? placed
        : placed.setMany(captured.map((p) => MapEntry(p, CellState.empty)));

    // Suicide check: own group must have liberties.
    final ownGroup = findGroup(afterCapture, point);
    if (ownGroup.liberties.isEmpty && !state.config.allowSuicide) {
      return MoveResult.rejected(MoveRejection.suicide);
    }

    // Superko: position must not repeat.
    final newHash = afterCapture.zobristHash();
    if (state.config.useSuperko && state.previousHashes.contains(newHash)) {
      return MoveResult.rejected(MoveRejection.superkoViolation);
    }

    // Simple ko marker: exactly one stone captured and the placed stone is alone with one liberty.
    Point? koPoint;
    if (captured.length == 1 &&
        ownGroup.stones.length == 1 &&
        ownGroup.liberties.length == 1) {
      koPoint = captured.first;
    }

    final move = Move(
      moveNumber: state.moveNumber + 1,
      player: player,
      type: MoveType.placeStone,
      point: point,
      captured: captured.toList(),
    );

    final capsByBlack = state.capturesByBlack +
        (player == StoneColor.black ? captured.length : 0);
    final capsByWhite = state.capturesByWhite +
        (player == StoneColor.white ? captured.length : 0);

    final next = state.copyWith(
      board: afterCapture,
      currentPlayer: opponent,
      moveNumber: state.moveNumber + 1,
      capturesByBlack: capsByBlack,
      capturesByWhite: capsByWhite,
      koPoint: koPoint,
      previousHashes: <int>{...state.previousHashes, newHash},
      consecutivePasses: 0,
      lastMove: move,
      history: [...state.history, move],
    );
    return MoveResult.accepted(next, move);
  }

  /// Returns the list of legal placement points for the current player.
  /// Pass and resign are always legal in [active] games.
  static List<Point> legalPlacements(GameState state) {
    if (state.status != GameStatus.active) return const [];
    final out = <Point>[];
    final s = state.board.size;
    for (var r = 0; r < s; r++) {
      for (var c = 0; c < s; c++) {
        final p = Point(r, c);
        if (state.board.cellAt(p) != CellState.empty) continue;
        final res = apply(state, MoveIntent.place(p));
        if (res.isAccepted) out.add(p);
      }
    }
    return out;
  }
}
