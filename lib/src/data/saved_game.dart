import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/rules.dart';
import '../domain/scoring.dart';

class SavedGameEntity {
  final String id;
  final int createdAtMillis;
  final int updatedAtMillis;
  final int boardSize;
  final double komi;
  final int handicap;
  final String ruleset;
  final String status;

  /// Encoded as a sequence of moves: e.g. "B,4,4|W,3,3|B,P|W,R".
  final String movesEncoded;
  final String opponentLabel;
  final String resultLabel;
  final String youColor;
  final String sgfPath;
  final double? blackTotal;
  final double? whiteTotal;

  const SavedGameEntity({
    required this.id,
    required this.createdAtMillis,
    required this.updatedAtMillis,
    required this.boardSize,
    required this.komi,
    required this.handicap,
    required this.status,
    required this.movesEncoded,
    this.ruleset = 'CHINESE',
    this.opponentLabel = 'Local',
    this.resultLabel = '',
    this.youColor = 'BLACK',
    this.sgfPath = '',
    this.blackTotal,
    this.whiteTotal,
  });

  Map<String, Object?> toRow() => {
        'id': id,
        'createdAtMillis': createdAtMillis,
        'updatedAtMillis': updatedAtMillis,
        'boardSize': boardSize,
        'komi': komi,
        'handicap': handicap,
        'ruleset': ruleset,
        'status': status,
        'movesEncoded': movesEncoded,
        'opponentLabel': opponentLabel,
        'resultLabel': resultLabel,
        'youColor': youColor,
        'sgfPath': sgfPath,
        'blackTotal': blackTotal,
        'whiteTotal': whiteTotal,
      };

  static SavedGameEntity fromRow(Map<String, Object?> r) => SavedGameEntity(
        id: r['id']! as String,
        createdAtMillis: (r['createdAtMillis']! as num).toInt(),
        updatedAtMillis: (r['updatedAtMillis']! as num).toInt(),
        boardSize: (r['boardSize']! as num).toInt(),
        komi: (r['komi']! as num).toDouble(),
        handicap: (r['handicap']! as num).toInt(),
        ruleset: r['ruleset'] as String? ?? 'CHINESE',
        status: r['status']! as String,
        movesEncoded: r['movesEncoded'] as String? ?? '',
        opponentLabel: r['opponentLabel'] as String? ?? 'Local',
        resultLabel: r['resultLabel'] as String? ?? '',
        youColor: r['youColor'] as String? ?? 'BLACK',
        sgfPath: r['sgfPath'] as String? ?? '',
        blackTotal: (r['blackTotal'] as num?)?.toDouble(),
        whiteTotal: (r['whiteTotal'] as num?)?.toDouble(),
      );
}

class GameSerializer {
  static String _statusName(GameStatus s) => switch (s) {
        GameStatus.active => 'ACTIVE',
        GameStatus.scoring => 'SCORING',
        GameStatus.completed => 'COMPLETED',
        GameStatus.resigned => 'RESIGNED',
      };

  static GameStatus _statusFrom(String s) => switch (s) {
        'ACTIVE' => GameStatus.active,
        'SCORING' => GameStatus.scoring,
        'COMPLETED' => GameStatus.completed,
        'RESIGNED' => GameStatus.resigned,
        _ => GameStatus.active,
      };

  static String encode(GameState state) => state.history.map((m) {
        final tag = m.player == StoneColor.black ? 'B' : 'W';
        switch (m.type) {
          case MoveType.pass:
            return '$tag,P';
          case MoveType.resign:
            return '$tag,R';
          case MoveType.placeStone:
            return '$tag,${m.point!.row},${m.point!.col}';
        }
      }).join('|');

  /// Replay encoded moves on top of a fresh game with the same config.
  static GameState decode(GameConfig config, String encoded) {
    var s = GameState.newGame(config);
    if (encoded.trim().isEmpty) return s;
    for (final token in encoded.split('|')) {
      final parts = token.split(',');
      MoveIntent? intent;
      if (parts.length == 2 && parts[1] == 'P') {
        intent = const MoveIntent.pass();
      } else if (parts.length == 2 && parts[1] == 'R') {
        intent = const MoveIntent.resign();
      } else if (parts.length == 3) {
        intent =
            MoveIntent.place(Point(int.parse(parts[1]), int.parse(parts[2])));
      }
      if (intent == null) continue;
      final res = Rules.apply(s, intent);
      if (res.isAccepted) {
        s = res.newStateAs<GameState>();
      } else {
        return s;
      }
    }
    return s;
  }

  static SavedGameEntity toEntity({
    required String id,
    required GameState state,
    required int createdAt,
    required int updatedAt,
    String opponentLabel = 'Local',
    String resultLabel = '',
    String youColor = 'BLACK',
    String sgfPath = '',
    ScoreResult? score,
  }) =>
      SavedGameEntity(
        id: id,
        createdAtMillis: createdAt,
        updatedAtMillis: updatedAt,
        boardSize: state.config.boardSize,
        komi: state.config.komi,
        handicap: state.config.handicap,
        ruleset: state.config.ruleset.name.toUpperCase(),
        status: _statusName(state.status),
        movesEncoded: encode(state),
        opponentLabel: opponentLabel,
        resultLabel: resultLabel,
        youColor: youColor,
        sgfPath: sgfPath,
        blackTotal: score?.blackTotal,
        whiteTotal: score?.whiteTotal,
      );

  static GameState fromEntity(SavedGameEntity e) {
    final ruleset = _rulesetFrom(e.ruleset);
    final defaults = RulesetDefaults.of(ruleset);
    final cfg = GameConfig(
      boardSize: e.boardSize,
      ruleset: ruleset,
      komi: e.komi,
      handicap: e.handicap,
      allowSuicide: defaults.allowSuicide,
      superkoMode: defaults.superkoMode,
    );
    var s = decode(cfg, e.movesEncoded);
    if (s.status == GameStatus.active &&
        _statusFrom(e.status) == GameStatus.scoring) {
      s = s.copyWith(status: GameStatus.scoring);
    }
    return s;
  }

  static Ruleset _rulesetFrom(String s) {
    for (final r in Ruleset.values) {
      if (r.name.toUpperCase() == s.toUpperCase()) return r;
    }
    return Ruleset.chinese;
  }
}
