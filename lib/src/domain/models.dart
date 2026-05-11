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

enum Ruleset {
  chinese,
  japanese,
  korean,
  aga,
  ing,
  newZealand,
  trompTaylor,
}

extension RulesetLabel on Ruleset {
  /// Human-readable label used across UI surfaces.
  String get label {
    switch (this) {
      case Ruleset.chinese:
        return 'Chinese';
      case Ruleset.japanese:
        return 'Japanese';
      case Ruleset.korean:
        return 'Korean';
      case Ruleset.aga:
        return 'AGA';
      case Ruleset.ing:
        return 'Ing';
      case Ruleset.newZealand:
        return 'New Zealand';
      case Ruleset.trompTaylor:
        return 'Tromp–Taylor';
    }
  }
}

enum ScoringMethod { area, territory }

enum SuperkoMode {
  /// Only the basic immediate-recapture ko is enforced (via [GameState.koPoint]).
  /// Used by Japanese / Korean rulesets.
  none,

  /// No board position may repeat at any point in the game.
  /// Used by Chinese / Tromp–Taylor.
  positional,

  /// No (position, player-to-move) may repeat. Slightly weaker than positional
  /// — the same board with different player-to-move is allowed.
  /// Used by AGA / NZ / Ing (approximation).
  situational,
}

class RulesetDefaults {
  final double komi;
  final bool allowSuicide;
  final SuperkoMode superkoMode;
  final ScoringMethod scoringMethod;
  const RulesetDefaults({
    required this.komi,
    required this.allowSuicide,
    required this.superkoMode,
    required this.scoringMethod,
  });

  static const _table = <Ruleset, RulesetDefaults>{
    Ruleset.chinese: RulesetDefaults(
      komi: 7.5,
      allowSuicide: false,
      superkoMode: SuperkoMode.positional,
      scoringMethod: ScoringMethod.area,
    ),
    Ruleset.japanese: RulesetDefaults(
      komi: 6.5,
      allowSuicide: false,
      superkoMode: SuperkoMode.none,
      scoringMethod: ScoringMethod.territory,
    ),
    Ruleset.korean: RulesetDefaults(
      komi: 6.5,
      allowSuicide: false,
      superkoMode: SuperkoMode.none,
      scoringMethod: ScoringMethod.territory,
    ),
    Ruleset.aga: RulesetDefaults(
      komi: 7.5,
      allowSuicide: false,
      superkoMode: SuperkoMode.situational,
      scoringMethod: ScoringMethod.area,
    ),
    Ruleset.ing: RulesetDefaults(
      komi: 8.0,
      allowSuicide: true,
      superkoMode: SuperkoMode.situational,
      scoringMethod: ScoringMethod.area,
    ),
    Ruleset.newZealand: RulesetDefaults(
      komi: 7.0,
      allowSuicide: true,
      superkoMode: SuperkoMode.situational,
      scoringMethod: ScoringMethod.area,
    ),
    Ruleset.trompTaylor: RulesetDefaults(
      komi: 7.5,
      allowSuicide: true,
      superkoMode: SuperkoMode.positional,
      scoringMethod: ScoringMethod.area,
    ),
  };

  static RulesetDefaults of(Ruleset r) => _table[r]!;
}

class GameConfig {
  final int boardSize;
  final Ruleset ruleset;
  final double komi;
  final int handicap;
  final bool allowSuicide;
  final SuperkoMode superkoMode;

  const GameConfig({
    required this.boardSize,
    this.ruleset = Ruleset.chinese,
    this.komi = 7.5,
    this.handicap = 0,
    this.allowSuicide = false,
    this.superkoMode = SuperkoMode.positional,
  });

  GameConfig copyWith({
    int? boardSize,
    Ruleset? ruleset,
    double? komi,
    int? handicap,
    bool? allowSuicide,
    SuperkoMode? superkoMode,
  }) =>
      GameConfig(
        boardSize: boardSize ?? this.boardSize,
        ruleset: ruleset ?? this.ruleset,
        komi: komi ?? this.komi,
        handicap: handicap ?? this.handicap,
        allowSuicide: allowSuicide ?? this.allowSuicide,
        superkoMode: superkoMode ?? this.superkoMode,
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
