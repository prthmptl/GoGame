import 'board.dart';
import 'models.dart';

class GameState {
  final Board board;
  final GameConfig config;
  final StoneColor currentPlayer;
  final int moveNumber;
  final int capturesByBlack;
  final int capturesByWhite;
  final Point? koPoint;
  final Set<int> previousHashes;
  final GameStatus status;
  final int consecutivePasses;
  final Move? lastMove;
  final List<Move> history;

  const GameState({
    required this.board,
    required this.config,
    required this.currentPlayer,
    required this.moveNumber,
    required this.capturesByBlack,
    required this.capturesByWhite,
    required this.koPoint,
    required this.previousHashes,
    required this.status,
    required this.consecutivePasses,
    required this.lastMove,
    required this.history,
  });

  GameState copyWith({
    Board? board,
    GameConfig? config,
    StoneColor? currentPlayer,
    int? moveNumber,
    int? capturesByBlack,
    int? capturesByWhite,
    Object? koPoint = _sentinel,
    Set<int>? previousHashes,
    GameStatus? status,
    int? consecutivePasses,
    Object? lastMove = _sentinel,
    List<Move>? history,
  }) {
    return GameState(
      board: board ?? this.board,
      config: config ?? this.config,
      currentPlayer: currentPlayer ?? this.currentPlayer,
      moveNumber: moveNumber ?? this.moveNumber,
      capturesByBlack: capturesByBlack ?? this.capturesByBlack,
      capturesByWhite: capturesByWhite ?? this.capturesByWhite,
      koPoint: identical(koPoint, _sentinel) ? this.koPoint : koPoint as Point?,
      previousHashes: previousHashes ?? this.previousHashes,
      status: status ?? this.status,
      consecutivePasses: consecutivePasses ?? this.consecutivePasses,
      lastMove:
          identical(lastMove, _sentinel) ? this.lastMove : lastMove as Move?,
      history: history ?? this.history,
    );
  }

  static const _sentinel = Object();

  static GameState newGame(GameConfig config) {
    final board = Board.empty(config.boardSize);
    final starting = config.handicap > 0 ? StoneColor.white : StoneColor.black;
    final withHandicap = _applyHandicap(board, config.handicap);
    return GameState(
      board: withHandicap,
      config: config,
      currentPlayer: starting,
      moveNumber: 0,
      capturesByBlack: 0,
      capturesByWhite: 0,
      koPoint: null,
      previousHashes: <int>{withHandicap.zobristHash()},
      status: GameStatus.active,
      consecutivePasses: 0,
      lastMove: null,
      history: const <Move>[],
    );
  }

  static Board _applyHandicap(Board board, int handicap) {
    if (handicap <= 0) return board;
    final pts = _handicapPoints(board.size, handicap);
    return board.setMany(pts.map((p) => MapEntry(p, CellState.black)));
  }

  static List<Point> _handicapPoints(int size, int n) {
    if (size != 9 && size != 13 && size != 19) return const [];
    final edge = size == 9 ? 2 : 3;
    final far = size - 1 - edge;
    final mid = size ~/ 2;
    final star = <Point>[
      Point(edge, edge),
      Point(far, far),
      Point(edge, far),
      Point(far, edge),
      Point(mid, mid),
      Point(mid, edge),
      Point(mid, far),
      Point(edge, mid),
      Point(far, mid),
    ];
    return star.take(n.clamp(0, 9)).toList();
  }
}
