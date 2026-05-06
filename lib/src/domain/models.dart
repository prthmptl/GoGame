enum StoneColor {
  black,
  white;

  StoneColor get other =>
      this == StoneColor.black ? StoneColor.white : StoneColor.black;
  String get short => this == StoneColor.black ? 'B' : 'W';
}

enum CellState {
  empty,
  black,
  white;

  static CellState of(StoneColor color) =>
      color == StoneColor.black ? CellState.black : CellState.white;
}

class Point {
  final int row;
  final int col;
  const Point(this.row, this.col);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is Point && other.row == row && other.col == col);

  @override
  int get hashCode => row * 1000003 + col;

  @override
  String toString() => '($row,$col)';
}

enum MoveType { placeStone, pass, resign }

class Move {
  final int moveNumber;
  final StoneColor player;
  final MoveType type;
  final Point? point;
  final List<Point> captured;

  const Move({
    required this.moveNumber,
    required this.player,
    required this.type,
    required this.point,
    required this.captured,
  });
}

enum Ruleset { chinese }

class GameConfig {
  final int boardSize;
  final Ruleset ruleset;
  final double komi;
  final int handicap;
  final bool allowSuicide;
  final bool useSuperko;

  const GameConfig({
    required this.boardSize,
    this.ruleset = Ruleset.chinese,
    this.komi = 7.5,
    this.handicap = 0,
    this.allowSuicide = false,
    this.useSuperko = true,
  });

  GameConfig copyWith({
    int? boardSize,
    Ruleset? ruleset,
    double? komi,
    int? handicap,
    bool? allowSuicide,
    bool? useSuperko,
  }) =>
      GameConfig(
        boardSize: boardSize ?? this.boardSize,
        ruleset: ruleset ?? this.ruleset,
        komi: komi ?? this.komi,
        handicap: handicap ?? this.handicap,
        allowSuicide: allowSuicide ?? this.allowSuicide,
        useSuperko: useSuperko ?? this.useSuperko,
      );
}

enum GameStatus { active, scoring, completed, resigned }

enum MoveRejection {
  gameNotActive,
  outOfBounds,
  occupied,
  suicide,
  koViolation,
  superkoViolation
}

/// Sealed result type. Use [accepted] / [rejected] factory constructors.
class MoveResult {
  final bool isAccepted;
  final dynamic _state; // GameState? — kept dynamic to avoid forward import.
  final Move? move;
  final MoveRejection? reason;

  const MoveResult._(this.isAccepted, this._state, this.move, this.reason);

  factory MoveResult.accepted(Object newState, Move move) =>
      MoveResult._(true, newState, move, null);
  factory MoveResult.rejected(MoveRejection reason) =>
      MoveResult._(false, null, null, reason);

  T newStateAs<T>() => _state as T;
}

class MoveIntent {
  final MoveType type;
  final Point? point;
  const MoveIntent(this.type, [this.point]);
  const MoveIntent.pass() : this(MoveType.pass);
  const MoveIntent.resign() : this(MoveType.resign);
  const MoveIntent.place(Point p) : this(MoveType.placeStone, p);
}
