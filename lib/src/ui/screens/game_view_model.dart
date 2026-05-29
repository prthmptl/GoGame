import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import '../../data/saved_game_repo.dart';
import '../../domain/ai/advanced_ai.dart';
import '../../domain/ai/beginner_ai.dart';
import '../../domain/ai/go_ai.dart';
import '../../domain/ai/intermediate_ai.dart';
import '../../domain/clock/clock_controller.dart';
import '../../domain/clock/time_control.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/rules.dart';
import '../../domain/scoring.dart';
import '../../sgf/sgf.dart';

enum Opponent { human, ai }

const _defaultTimeControl = TimeControl.absolute(mainSeconds: 10 * 60);

class GameUi {
  final GameState state;
  final String? rejection;
  final Set<Point> deadStones;
  final ScoreResult? score;
  final bool aiThinking;
  final Opponent opponent;
  final StoneColor aiPlays;
  final AiDifficulty aiDifficulty;
  final String? botName;
  final String? sgf;
  final TimeControl timeControl;
  final ClockSnapshot blackClock;
  final ClockSnapshot whiteClock;
  final StoneColor? timeoutLoser;
  final Point? pendingPoint;
  final bool showHints;

  const GameUi({
    required this.state,
    this.rejection,
    this.deadStones = const {},
    this.score,
    this.aiThinking = false,
    this.opponent = Opponent.human,
    this.aiPlays = StoneColor.white,
    this.aiDifficulty = AiDifficulty.beginner,
    this.botName,
    this.sgf,
    required this.timeControl,
    required this.blackClock,
    required this.whiteClock,
    this.timeoutLoser,
    this.pendingPoint,
    this.showHints = false,
  });

  GameUi copyWith({
    GameState? state,
    Object? rejection = _sentinel,
    Set<Point>? deadStones,
    Object? score = _sentinel,
    bool? aiThinking,
    Opponent? opponent,
    StoneColor? aiPlays,
    AiDifficulty? aiDifficulty,
    Object? botName = _sentinel,
    Object? sgf = _sentinel,
    TimeControl? timeControl,
    ClockSnapshot? blackClock,
    ClockSnapshot? whiteClock,
    Object? timeoutLoser = _sentinel,
    Object? pendingPoint = _sentinel,
    bool? showHints,
  }) =>
      GameUi(
        state: state ?? this.state,
        rejection: identical(rejection, _sentinel)
            ? this.rejection
            : rejection as String?,
        deadStones: deadStones ?? this.deadStones,
        score: identical(score, _sentinel) ? this.score : score as ScoreResult?,
        aiThinking: aiThinking ?? this.aiThinking,
        opponent: opponent ?? this.opponent,
        aiPlays: aiPlays ?? this.aiPlays,
        aiDifficulty: aiDifficulty ?? this.aiDifficulty,
        botName:
            identical(botName, _sentinel) ? this.botName : botName as String?,
        sgf: identical(sgf, _sentinel) ? this.sgf : sgf as String?,
        timeControl: timeControl ?? this.timeControl,
        blackClock: blackClock ?? this.blackClock,
        whiteClock: whiteClock ?? this.whiteClock,
        timeoutLoser: identical(timeoutLoser, _sentinel)
            ? this.timeoutLoser
            : timeoutLoser as StoneColor?,
        pendingPoint: identical(pendingPoint, _sentinel)
            ? this.pendingPoint
            : pendingPoint as Point?,
        showHints: showHints ?? this.showHints,
      );

  static const _sentinel = Object();
}

/// Lightweight ChangeNotifier-based viewmodel; mirrors the original Kotlin GameViewModel.
class GameViewModel extends ChangeNotifier {
  final SavedGameRepo? repo;
  GoAi _ai = BeginnerAi();
  ClockController _clock = ClockController(_defaultTimeControl);

  GameUi _ui = GameUi(
    state: GameState.newGame(const GameConfig(boardSize: 9)),
    timeControl: _defaultTimeControl,
    blackClock: ClockController(_defaultTimeControl).black,
    whiteClock: ClockController(_defaultTimeControl).white,
  );
  Timer? _clockTimer;
  int _lastTickMillis = 0;

  GameViewModel({this.repo});

  GameUi get ui => _ui;

  void _set(GameUi next) {
    _ui = next;
    notifyListeners();
  }

  String _opponentLabel(GameUi ui) {
    if (ui.opponent != Opponent.ai) return 'Local';
    final name = ui.botName;
    if (name != null && name.isNotEmpty) return 'Practice · $name';
    return 'Practice · ${ui.aiDifficulty.label}';
  }

  StoneColor _youColor(GameUi ui) =>
      ui.opponent == Opponent.ai ? ui.aiPlays.other : StoneColor.black;

  void _autosave() {
    final cur = _ui;
    if (repo == null) return;
    unawaited(repo!.saveCurrent(
      state: cur.state,
      opponentLabel: _opponentLabel(cur),
      youColor: _youColor(cur),
    ));
  }

  void _archiveAndClear() {
    final cur = _ui;
    if (repo == null) return;
    final resultLabel = _computeResultLabel(cur);
    unawaited(() async {
      await repo!.archiveCompleted(
        state: cur.state,
        opponentLabel: _opponentLabel(cur),
        youColor: _youColor(cur),
        resultLabel: resultLabel,
        score: cur.score,
      );
      await repo!.clearCurrent();
    }());
  }

  String _computeResultLabel(GameUi ui) {
    if (ui.timeoutLoser != null) return '${ui.timeoutLoser!.other.short}+T';
    switch (ui.state.status) {
      case GameStatus.resigned:
        if (ui.state.history.isEmpty) return '';
        final loser = ui.state.history.last.player;
        return '${loser.other.short}+R';
      case GameStatus.completed:
        return ui.score?.resultString ?? '';
      default:
        return '';
    }
  }

  void startGame({
    required GameConfig config,
    required Opponent opponent,
    StoneColor aiPlays = StoneColor.white,
    AiDifficulty aiDifficulty = AiDifficulty.beginner,
    TimeControl timeControl = _defaultTimeControl,
    String? botName,
    bool showHints = false,
  }) {
    _ai = _buildAi(aiDifficulty);
    _clock = ClockController(timeControl, active: StoneColor.black);
    _set(GameUi(
      state: GameState.newGame(config),
      opponent: opponent,
      aiPlays: aiPlays,
      aiDifficulty: aiDifficulty,
      botName: botName,
      timeControl: timeControl,
      blackClock: _clock.black,
      whiteClock: _clock.white,
      showHints: showHints,
    ));
    _startClock();
    _maybeTriggerAi();
  }

  void loadGame(
    GameState state, {
    Opponent opponent = Opponent.human,
    AiDifficulty aiDifficulty = AiDifficulty.beginner,
    TimeControl timeControl = _defaultTimeControl,
  }) {
    _ai = _buildAi(aiDifficulty);
    _clock = ClockController(timeControl, active: state.currentPlayer);
    _set(GameUi(
      state: state,
      opponent: opponent,
      aiDifficulty: aiDifficulty,
      timeControl: timeControl,
      blackClock: _clock.black,
      whiteClock: _clock.white,
    ));
    _startClock();
  }

  Future<bool> resumeCurrent() async {
    final entity = await repo?.loadCurrentEntity();
    if (entity == null) return false;
    final state = await repo?.loadCurrent();
    if (state == null) return false;
    if (state.status == GameStatus.completed ||
        state.status == GameStatus.resigned) {
      return false;
    }
    final isAi = entity.opponentLabel.startsWith('Practice') ||
        entity.opponentLabel.startsWith('AI');
    final aiDifficulty = _difficultyFromLabel(entity.opponentLabel);
    final youColor = _stoneColorFromLabel(entity.youColor);
    _ai = _buildAi(aiDifficulty);
    _clock = ClockController(_defaultTimeControl, active: state.currentPlayer);
    _set(GameUi(
      state: state,
      opponent: isAi ? Opponent.ai : Opponent.human,
      aiPlays: isAi ? youColor.other : StoneColor.white,
      aiDifficulty: aiDifficulty,
      timeControl: _defaultTimeControl,
      blackClock: _clock.black,
      whiteClock: _clock.white,
    ));
    _startClock();
    _maybeTriggerAi();
    return true;
  }

  void tap(Point point) {
    final cur = _ui;
    if (cur.state.status == GameStatus.scoring) {
      toggleDead(point);
      return;
    }
    if (cur.state.status != GameStatus.active) return;
    if (cur.opponent == Opponent.ai && cur.state.currentPlayer == cur.aiPlays) {
      return;
    }

    if (cur.showHints) {
      if (cur.pendingPoint == point) {
        _set(cur.copyWith(pendingPoint: null));
        _play(MoveIntent.place(point));
      } else {
        final res = Rules.apply(cur.state, MoveIntent.place(point));
        if (!res.isAccepted) {
          _set(cur.copyWith(
              pendingPoint: null, rejection: _humanizeReason(res.reason!)));
        } else {
          _set(cur.copyWith(pendingPoint: point, rejection: null));
        }
      }
    } else {
      _play(MoveIntent.place(point));
    }
  }

  void cancelPending() {
    _set(_ui.copyWith(pendingPoint: null));
  }

  void pass() => _play(const MoveIntent.pass());
  void resign() => _play(const MoveIntent.resign());

  void undo() {
    final cur = _ui.state;
    if (cur.history.isEmpty) return;
    final isAi = _ui.opponent == Opponent.ai;
    final drop =
        isAi && cur.history.isNotEmpty && cur.history.last.player == _ui.aiPlays
            ? 2
            : 1;
    final newHistory =
        cur.history.sublist(0, math.max(0, cur.history.length - drop));
    var s = GameState.newGame(cur.config);
    for (final m in newHistory) {
      final intent = switch (m.type) {
        MoveType.pass => const MoveIntent.pass(),
        MoveType.resign => const MoveIntent.resign(),
        MoveType.placeStone => MoveIntent.place(m.point!),
      };
      final r = Rules.apply(s, intent);
      if (r.isAccepted) s = r.newStateAs<GameState>();
    }
    _set(_ui.copyWith(
        state: s, rejection: null, score: null, pendingPoint: null));
  }

  void _play(MoveIntent intent) {
    final cur = _ui;
    final res = Rules.apply(cur.state, intent);
    if (!res.isAccepted) {
      _set(cur.copyWith(rejection: _humanizeReason(res.reason!)));
      return;
    }
    final next = res.newStateAs<GameState>();
    final mover = cur.state.currentPlayer;
    final isAiMove = cur.opponent == Opponent.ai && mover == cur.aiPlays;
    if (intent.type == MoveType.placeStone && !isAiMove) {
      final prevCaps = cur.state.capturesByBlack + cur.state.capturesByWhite;
      final newCaps = next.capturesByBlack + next.capturesByWhite;
      if (newCaps > prevCaps) {
        HapticFeedback.mediumImpact();
      } else {
        HapticFeedback.lightImpact();
      }
    }
    _clock.onMovePlayed(mover);
    _set(cur.copyWith(
      state: next,
      rejection: null,
      pendingPoint: null,
      blackClock: _clock.black,
      whiteClock: _clock.white,
    ));
    switch (next.status) {
      case GameStatus.scoring:
        _stopClock();
        _computeScore();
        _autosave();
        break;
      case GameStatus.resigned:
      case GameStatus.completed:
        _stopClock();
        _archiveAndClear();
        break;
      case GameStatus.active:
        _autosave();
        _maybeTriggerAi();
        break;
    }
  }

  void _maybeTriggerAi() {
    final cur = _ui;
    if (cur.opponent != Opponent.ai) return;
    if (cur.state.status != GameStatus.active) return;
    if (cur.state.currentPlayer != cur.aiPlays) return;
    _set(cur.copyWith(aiThinking: true));
    final snapshot = cur.state;
    Future<void>(() async {
      // Run AI on a microtask boundary to keep the UI responsive.
      // The search is cheap on small boards; an isolate is unnecessary here.
      await Future<void>.delayed(const Duration(milliseconds: 1));
      final intent = _ai.chooseMove(snapshot);
      _set(_ui.copyWith(aiThinking: false));
      _play(intent);
    });
  }

  GoAi _buildAi(AiDifficulty difficulty) {
    switch (difficulty) {
      case AiDifficulty.beginner:
        return BeginnerAi();
      case AiDifficulty.intermediate:
        return IntermediateAi();
      case AiDifficulty.advanced:
        return AdvancedAi();
    }
  }

  AiDifficulty _difficultyFromLabel(String label) {
    for (final difficulty in AiDifficulty.values) {
      if (label.toLowerCase().contains(difficulty.label.toLowerCase())) {
        return difficulty;
      }
    }
    return AiDifficulty.beginner;
  }

  StoneColor _stoneColorFromLabel(String value) =>
      value.toUpperCase() == 'WHITE' ? StoneColor.white : StoneColor.black;

  void toggleDead(Point p) {
    final cur = _ui;
    if (cur.state.status != GameStatus.scoring) return;
    if (cur.state.board.cellAt(p) == CellState.empty) return;
    final next = cur.deadStones.contains(p)
        ? (cur.deadStones.toSet()..remove(p))
        : (cur.deadStones.toSet()..add(p));
    _set(cur.copyWith(deadStones: next));
    _computeScore();
  }

  void confirmScore() {
    final cur = _ui;
    final ended = cur.state.copyWith(status: GameStatus.completed);
    _set(cur.copyWith(
        state: ended, sgf: Sgf.export(cur.state, score: cur.score)));
    _stopClock();
    _archiveAndClear();
  }

  void resumePlay() {
    final cur = _ui;
    _set(cur.copyWith(
      state:
          cur.state.copyWith(status: GameStatus.active, consecutivePasses: 0),
      deadStones: const {},
      score: null,
    ));
    _startClock();
  }

  String exportSgf() {
    final cur = _ui;
    final sgf = Sgf.export(cur.state, score: cur.score);
    _set(cur.copyWith(sgf: sgf));
    return sgf;
  }

  void _computeScore() {
    final cur = _ui;
    final score = Scoring.score(cur.state, deadStones: cur.deadStones);
    _set(cur.copyWith(score: score));
  }

  String _humanizeReason(MoveRejection r) => switch (r) {
        MoveRejection.gameNotActive => 'Game is not active',
        MoveRejection.outOfBounds => 'Off the board',
        MoveRejection.occupied => 'Point already occupied',
        MoveRejection.suicide => 'Suicide is not allowed',
        MoveRejection.koViolation => 'Ko: cannot retake immediately',
        MoveRejection.superkoViolation => 'Superko: position would repeat',
      };

  // ---- Clock ----

  void _startClock() {
    _stopClock();
    _lastTickMillis = DateTime.now().millisecondsSinceEpoch;
    _clockTimer =
        Timer.periodic(const Duration(milliseconds: 250), (_) => _tick());
  }

  void _tick() {
    final now = DateTime.now().millisecondsSinceEpoch;
    final delta = now - _lastTickMillis;
    _lastTickMillis = now;
    final cur = _ui;
    if (cur.state.status != GameStatus.active) return;
    if (cur.timeControl.kind == TimeControlKind.none) return;
    _clock.switchActive(cur.state.currentPlayer);
    final flagged = _clock.tick(delta);
    StoneColor? timeoutLoser = cur.timeoutLoser ?? flagged;
    final newStatus =
        timeoutLoser != null ? GameStatus.completed : cur.state.status;
    _set(cur.copyWith(
      blackClock: _clock.black,
      whiteClock: _clock.white,
      timeoutLoser: timeoutLoser,
      state: newStatus != cur.state.status
          ? cur.state.copyWith(status: newStatus)
          : cur.state,
    ));
    if (_ui.timeoutLoser != null && _ui.state.status == GameStatus.completed) {
      _archiveAndClear();
      _stopClock();
    }
  }

  void _stopClock() {
    _clockTimer?.cancel();
    _clockTimer = null;
  }

  @override
  void dispose() {
    _stopClock();
    super.dispose();
  }
}
