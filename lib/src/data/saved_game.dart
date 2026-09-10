import 'dart:convert';

import '../domain/clock/clock_controller.dart';
import '../domain/clock/time_control.dart';
import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/rules.dart';
import '../domain/scoring.dart';

class SavedGameEntity {
  final String runtimeJson;
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
  final String gameVariant;
  final String botName;
  final String botStyle;
  final String aiDifficulty;
  final String timeControlKind;
  final int timeMainSeconds;
  final int timeIncrementSeconds;
  final int timePeriodSeconds;
  final int timePeriods;
  final int timeStonesPerPeriod;

  const SavedGameEntity({
    this.runtimeJson = '{}',
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
    this.gameVariant = 'standard',
    this.botName = '',
    this.botStyle = '',
    this.aiDifficulty = 'beginner',
    this.timeControlKind = 'absolute',
    this.timeMainSeconds = 600,
    this.timeIncrementSeconds = 0,
    this.timePeriodSeconds = 0,
    this.timePeriods = 0,
    this.timeStonesPerPeriod = 0,
  });

  Map<String, Object?> toRow() => {
        'runtimeJson': runtimeJson,
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
        'gameVariant': gameVariant,
        'botName': botName,
        'botStyle': botStyle,
        'aiDifficulty': aiDifficulty,
        'timeControlKind': timeControlKind,
        'timeMainSeconds': timeMainSeconds,
        'timeIncrementSeconds': timeIncrementSeconds,
        'timePeriodSeconds': timePeriodSeconds,
        'timePeriods': timePeriods,
        'timeStonesPerPeriod': timeStonesPerPeriod,
      };

  Map<String, dynamic> get runtime =>
      jsonDecode(runtimeJson) as Map<String, dynamic>;

  ClockSnapshot? clockFor(String color) {
    final value = runtime[color];
    return value == null
        ? null
        : ClockSnapshot.fromJson(value as Map<String, dynamic>);
  }

  Set<Point> get deadStones =>
      ((runtime['deadStones'] as List<dynamic>?) ?? []).map((p) {
        final pair = p as List<dynamic>;
        if (pair.length != 2) {
          throw const FormatException('Invalid saved dead stone');
        }
        final point = Point(pair[0] as int, pair[1] as int);
        if (point.row < 0 ||
            point.row >= boardSize ||
            point.col < 0 ||
            point.col >= boardSize) {
          throw const FormatException('Invalid saved dead stone');
        }
        return point;
      }).toSet();

  static SavedGameEntity fromRow(Map<String, Object?> r) => SavedGameEntity(
        runtimeJson: r['runtimeJson'] as String? ?? '{}',
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
        gameVariant: r['gameVariant'] as String? ?? 'standard',
        botName: r['botName'] as String? ?? '',
        botStyle: r['botStyle'] as String? ?? '',
        aiDifficulty: r['aiDifficulty'] as String? ?? 'beginner',
        timeControlKind: r['timeControlKind'] as String? ?? 'absolute',
        timeMainSeconds: (r['timeMainSeconds'] as num?)?.toInt() ?? 600,
        timeIncrementSeconds: (r['timeIncrementSeconds'] as num?)?.toInt() ?? 0,
        timePeriodSeconds: (r['timePeriodSeconds'] as num?)?.toInt() ?? 0,
        timePeriods: (r['timePeriods'] as num?)?.toInt() ?? 0,
        timeStonesPerPeriod: (r['timeStonesPerPeriod'] as num?)?.toInt() ?? 0,
      );
}

class GameSerializer {
  static const _defaultTimeControl = TimeControl.absolute(mainSeconds: 10 * 60);

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
      if (intent == null || (parts[0] != 'B' && parts[0] != 'W')) {
        throw const FormatException('Invalid saved move');
      }
      if (s.status == GameStatus.scoring) {
        s = s.copyWith(status: GameStatus.active, consecutivePasses: 0);
      }
      final player = parts[0] == 'B' ? StoneColor.black : StoneColor.white;
      if (intent.type == MoveType.resign) s = s.copyWith(currentPlayer: player);
      if (player != s.currentPlayer) {
        throw const FormatException('Wrong player in saved game');
      }
      final res = Rules.apply(s, intent);
      if (res.isAccepted) {
        s = res.newStateAs<GameState>();
      } else {
        throw FormatException('Illegal saved move: ${res.reason}');
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
    TimeControl? timeControl,
    String? botName,
    String? botStyle,
    AiDifficulty aiDifficulty = AiDifficulty.beginner,
    ClockSnapshot? blackClock,
    ClockSnapshot? whiteClock,
    Set<Point> deadStones = const {},
  }) =>
      _toEntity(
        id: id,
        state: state,
        createdAt: createdAt,
        updatedAt: updatedAt,
        opponentLabel: opponentLabel,
        resultLabel: resultLabel,
        youColor: youColor,
        sgfPath: sgfPath,
        score: score,
        timeControl: timeControl ?? _defaultTimeControl,
        botName: botName,
        botStyle: botStyle,
        aiDifficulty: aiDifficulty,
        blackClock: blackClock,
        whiteClock: whiteClock,
        deadStones: deadStones,
      );

  static SavedGameEntity _toEntity({
    required String id,
    required GameState state,
    required int createdAt,
    required int updatedAt,
    required String opponentLabel,
    required String resultLabel,
    required String youColor,
    required String sgfPath,
    required ScoreResult? score,
    required TimeControl timeControl,
    required String? botName,
    required String? botStyle,
    required AiDifficulty aiDifficulty,
    required ClockSnapshot? blackClock,
    required ClockSnapshot? whiteClock,
    required Set<Point> deadStones,
  }) =>
      SavedGameEntity(
        runtimeJson: jsonEncode({
          'allowSuicide': state.config.allowSuicide,
          'superkoMode': state.config.superkoMode.name,
          'consecutivePasses': state.consecutivePasses,
          if (blackClock != null) 'black': blackClock.toJson(),
          if (whiteClock != null) 'white': whiteClock.toJson(),
          'deadStones': deadStones.map((p) => [p.row, p.col]).toList(),
        }),
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
        gameVariant: state.config.variant.name,
        botName: botName ?? '',
        botStyle: botStyle ?? '',
        aiDifficulty: aiDifficulty.name,
        timeControlKind: timeControl.kind.name,
        timeMainSeconds: timeControl.mainSeconds,
        timeIncrementSeconds: timeControl.incrementSeconds,
        timePeriodSeconds: timeControl.periodSeconds,
        timePeriods: timeControl.periods,
        timeStonesPerPeriod: timeControl.stonesPerPeriod,
      );

  static GameState fromEntity(SavedGameEntity e) {
    if (e.boardSize < 2 ||
        e.boardSize > 25 ||
        !e.komi.isFinite ||
        e.handicap < 0 ||
        e.handicap > 9) {
      throw const FormatException('Invalid saved game configuration');
    }
    final ruleset = _rulesetFrom(e.ruleset);
    final defaults = RulesetDefaults.of(ruleset);
    final cfg = GameConfig(
      boardSize: e.boardSize,
      ruleset: ruleset,
      komi: e.komi,
      handicap: e.handicap,
      allowSuicide: e.runtime['allowSuicide'] as bool? ?? defaults.allowSuicide,
      superkoMode: SuperkoMode.values.firstWhere(
          (mode) => mode.name == e.runtime['superkoMode'],
          orElse: () => defaults.superkoMode),
      variant: _gameVariantFrom(e.gameVariant),
    );
    var s = decode(cfg, e.movesEncoded);
    final status = _statusFrom(e.status);
    s = s.copyWith(
        status: status,
        consecutivePasses: e.runtime['consecutivePasses'] as int? ??
            (status == GameStatus.active && s.status == GameStatus.scoring
                ? 0
                : s.consecutivePasses));
    return s;
  }

  static TimeControl timeControlFromEntity(SavedGameEntity e) {
    final kind = _timeKindFrom(e.timeControlKind);
    return switch (kind) {
      TimeControlKind.none => const TimeControl.none(),
      TimeControlKind.absolute =>
        TimeControl.absolute(mainSeconds: e.timeMainSeconds),
      TimeControlKind.fischer => TimeControl.fischer(
          mainSeconds: e.timeMainSeconds,
          incrementSeconds: e.timeIncrementSeconds,
        ),
      TimeControlKind.byoYomi => TimeControl.byoYomi(
          mainSeconds: e.timeMainSeconds,
          periods: e.timePeriods,
          periodSeconds: e.timePeriodSeconds,
        ),
      TimeControlKind.canadian => TimeControl.canadian(
          mainSeconds: e.timeMainSeconds,
          stonesPerPeriod: e.timeStonesPerPeriod,
          periodSeconds: e.timePeriodSeconds,
        ),
    };
  }

  static GameVariant _gameVariantFrom(String value) {
    for (final variant in GameVariant.values) {
      if (variant.name.toLowerCase() == value.toLowerCase()) return variant;
    }
    return GameVariant.standard;
  }

  static TimeControlKind _timeKindFrom(String value) {
    for (final kind in TimeControlKind.values) {
      if (kind.name.toLowerCase() == value.toLowerCase()) return kind;
    }
    return TimeControlKind.absolute;
  }

  static Ruleset _rulesetFrom(String s) {
    for (final r in Ruleset.values) {
      if (r.name.toUpperCase() == s.toUpperCase()) return r;
    }
    return Ruleset.chinese;
  }
}
