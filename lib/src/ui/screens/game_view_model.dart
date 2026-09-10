import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart'
    show WidgetsBindingObserver, WidgetsBinding, AppLifecycleState;

import '../../data/saved_game.dart';
import '../../data/saved_game_repo.dart';
import '../../domain/ai/advanced_ai.dart';
import '../../domain/ai/beginner_ai.dart';
import '../../domain/ai/go_ai.dart';
import '../../domain/ai/intermediate_ai.dart';
import '../../domain/ai/styled_ai.dart';
import '../../domain/bots/bot_profile.dart';
import '../../domain/clock/clock_controller.dart';
import '../../domain/clock/time_control.dart';
import '../../domain/game_state.dart';
import '../../domain/groups.dart';
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
  final BotStyle? botStyle;
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
    this.botStyle,
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
    Object? botStyle = _sentinel,
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
        botStyle: identical(botStyle, _sentinel)
            ? this.botStyle
            : botStyle as BotStyle?,
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
class GameViewModel extends ChangeNotifier with WidgetsBindingObserver {
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
  int _lastSavedMillis = 0;
  final Stopwatch _elapsed = Stopwatch()..start();
  final int Function()? _nowMillis;
  int get _now => _nowMillis?.call() ?? _elapsed.elapsedMilliseconds;
  int _generation = 0;
  bool _disposed = false;
  bool _paused = false;
  bool _visible = true;
  Future<void> _persistence = Future<void>.value();
  final Future<MoveIntent> Function(GoAi, GameState) _chooseMove;

  GameViewModel(
      {this.repo,
      Future<MoveIntent> Function(GoAi, GameState)? chooseMove,
      int Function()? nowMillis})
      : _chooseMove = chooseMove ?? _runAi,
        _nowMillis = nowMillis {
    WidgetsBinding.instance.addObserver(this);
  }

  Future<void> get saved => _persistence;

  void _persist(Future<void> Function() action) {
    _persistence = _persistence
        .then((_) => action())
        .catchError((Object error, StackTrace stack) {
      debugPrint('Game save failed: $error');
      if (!_disposed) {
        _set(_ui.copyWith(
            rejection: 'Could not save the game. Check available storage.'));
      }
    });
  }

  GameUi get ui => _ui;

  void _set(GameUi next) {
    if (_disposed) return;
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
    _persist(() => repo!.saveCurrent(
          state: cur.state,
          opponentLabel: _opponentLabel(cur),
          youColor: _youColor(cur),
          timeControl: cur.timeControl,
          botName: cur.botName,
          botStyle: cur.botStyle?.name,
          aiDifficulty: cur.aiDifficulty,
          blackClock: cur.blackClock,
          whiteClock: cur.whiteClock,
          deadStones: cur.deadStones,
        ));
  }

  void _archiveAndClear() {
    final cur = _ui;
    if (repo == null) return;
    final resultLabel = _computeResultLabel(cur);
    _persist(() async {
      await repo!.archiveCompleted(
        state: cur.state,
        opponentLabel: _opponentLabel(cur),
        youColor: _youColor(cur),
        resultLabel: resultLabel,
        score: cur.score,
        timeControl: cur.timeControl,
        botName: cur.botName,
        botStyle: cur.botStyle?.name,
        aiDifficulty: cur.aiDifficulty,
      );
    });
  }

  String _computeResultLabel(GameUi ui) {
    if (ui.timeoutLoser != null) return '${ui.timeoutLoser!.other.short}+T';
    switch (ui.state.status) {
      case GameStatus.resigned:
        if (ui.state.history.isEmpty) return '';
        final loser = ui.state.history.last.player;
        return '${loser.other.short}+R';
      case GameStatus.completed:
        if (ui.state.config.variant == GameVariant.atariGo &&
            ui.state.lastMove?.captured.isNotEmpty == true) {
          return '${ui.state.lastMove!.player.short}+Capture';
        }
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
    BotStyle? botStyle,
    bool showHints = false,
  }) {
    _generation++;
    _paused = false;
    _visible = true;
    if (repo != null) _persist(() => repo!.clearCurrent());
    _ai = _buildAi(aiDifficulty, botStyle: botStyle);
    final state = GameState.newGame(config);
    _clock = ClockController(timeControl, active: state.currentPlayer);
    _set(GameUi(
      state: state,
      opponent: opponent,
      aiPlays: aiPlays,
      aiDifficulty: aiDifficulty,
      botName: botName,
      botStyle: botStyle,
      timeControl: timeControl,
      blackClock: _clock.black,
      whiteClock: _clock.white,
      showHints: showHints,
    ));
    _startClock();
    _autosave();
    _maybeTriggerAi();
  }

  void loadGame(
    GameState state, {
    Opponent opponent = Opponent.human,
    AiDifficulty aiDifficulty = AiDifficulty.beginner,
    TimeControl timeControl = _defaultTimeControl,
    String? botName,
    BotStyle? botStyle,
  }) {
    _generation++;
    _paused = false;
    _ai = _buildAi(aiDifficulty, botStyle: botStyle);
    _clock = ClockController(timeControl, active: state.currentPlayer);
    _set(GameUi(
      state: state,
      opponent: opponent,
      aiDifficulty: aiDifficulty,
      botName: botName,
      botStyle: botStyle,
      timeControl: timeControl,
      blackClock: _clock.black,
      whiteClock: _clock.white,
    ));
    if (state.status == GameStatus.scoring) _computeScore();
    _startClock();
    _maybeTriggerAi();
  }

  Future<bool> resumeCurrent() async {
    final generation = ++_generation;
    try {
      await saved;
      final entity = await repo?.loadCurrentEntity();
      if (_disposed || generation != _generation) return false;
      if (entity == null) return false;
      final state = GameSerializer.fromEntity(entity);
      if (state.status == GameStatus.completed ||
          state.status == GameStatus.resigned) {
        return false;
      }
      final isAi = entity.opponentLabel.startsWith('Practice') ||
          entity.opponentLabel.startsWith('AI');
      final aiDifficulty = _difficultyFromLabel(
        entity.aiDifficulty.isNotEmpty
            ? entity.aiDifficulty
            : entity.opponentLabel,
      );
      final youColor = _stoneColorFromLabel(entity.youColor);
      final timeControl = GameSerializer.timeControlFromEntity(entity);
      final botStyle = _botStyleFromLabel(entity.botStyle);
      final botName = entity.botName.isNotEmpty
          ? entity.botName
          : _botNameFromOpponentLabel(entity.opponentLabel);
      _ai = _buildAi(aiDifficulty, botStyle: botStyle);
      _paused = false;
      final black = entity.clockFor('black');
      final white = entity.clockFor('white');
      _clock = black != null && white != null
          ? ClockController.restore(timeControl,
              black: black, white: white, active: state.currentPlayer)
          : ClockController(timeControl, active: state.currentPlayer);
      _set(GameUi(
        state: state,
        deadStones: entity.deadStones,
        opponent: isAi ? Opponent.ai : Opponent.human,
        aiPlays: isAi ? youColor.other : StoneColor.white,
        aiDifficulty: aiDifficulty,
        botName: isAi ? botName : null,
        botStyle: isAi ? botStyle : null,
        timeControl: timeControl,
        blackClock: _clock.black,
        whiteClock: _clock.white,
      ));
      if (state.status == GameStatus.scoring) _computeScore();
      _startClock();
      _maybeTriggerAi();
      return true;
    } on Object {
      if (!_disposed && generation == _generation) {
        _set(_ui.copyWith(rejection: 'The saved game could not be read.'));
      }
      return false;
    }
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

  void pass() {
    if (_paused ||
        (_ui.opponent == Opponent.ai &&
            _ui.state.currentPlayer == _ui.aiPlays)) {
      return;
    }
    _play(const MoveIntent.pass());
  }

  void resign() {
    if (_ui.state.status != GameStatus.active &&
        _ui.state.status != GameStatus.scoring) {
      return;
    }
    _tick();
    if (_ui.state.status == GameStatus.completed) return;
    _generation++;
    final loser = _ui.opponent == Opponent.ai
        ? _ui.aiPlays.other
        : _ui.state.currentPlayer;
    _set(_ui.copyWith(
        state:
            _ui.state.copyWith(currentPlayer: loser, status: GameStatus.active),
        aiThinking: false));
    _play(const MoveIntent.resign());
  }

  void undo() {
    final cur = _ui.state;
    if (cur.history.isEmpty ||
        (cur.status != GameStatus.active && cur.status != GameStatus.scoring)) {
      return;
    }
    _tick();
    if (_ui.state.status == GameStatus.completed) return;
    _generation++;
    final isAi = _ui.opponent == Opponent.ai;
    final drop =
        isAi && cur.history.isNotEmpty && cur.history.last.player == _ui.aiPlays
            ? 2
            : 1;
    final newHistory =
        cur.history.sublist(0, math.max(0, cur.history.length - drop));
    final state = GameSerializer.decode(
        cur.config, GameSerializer.encode(cur.copyWith(history: newHistory)));
    _clock.switchActive(state.currentPlayer);
    _set(_ui.copyWith(
        state: state,
        rejection: null,
        score: null,
        deadStones: const {},
        pendingPoint: null,
        aiThinking: false,
        sgf: null));
    _startClock();
    _autosave();
    _maybeTriggerAi();
  }

  void _play(MoveIntent intent) {
    if (_disposed || _paused) return;
    _tick();
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
    if (intent.type != MoveType.resign) _clock.onMovePlayed(mover);
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
        _generation++;
        _set(_ui.copyWith(aiThinking: false));
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
    if (_disposed ||
        _paused ||
        cur.aiThinking ||
        cur.opponent != Opponent.ai ||
        cur.state.status != GameStatus.active ||
        cur.state.currentPlayer != cur.aiPlays) {
      return;
    }
    final generation = _generation;
    final snapshot = cur.state;
    final ai = _ai;
    _set(cur.copyWith(aiThinking: true));
    unawaited(() async {
      try {
        final intent = await _chooseMove(ai, snapshot);
        if (_disposed ||
            generation != _generation ||
            !identical(_ui.state, snapshot)) {
          return;
        }
        _set(_ui.copyWith(aiThinking: false));
        _play(intent);
      } catch (error) {
        if (_disposed || generation != _generation) return;
        _set(_ui.copyWith(
            aiThinking: false,
            rejection: 'AI could not finish its turn. Undo to try again.'));
      }
    }());
  }

  GoAi _buildAi(AiDifficulty difficulty, {BotStyle? botStyle}) {
    final base = switch (difficulty) {
      AiDifficulty.beginner => BeginnerAi(),
      AiDifficulty.intermediate => IntermediateAi(),
      AiDifficulty.advanced => AdvancedAi(),
    };
    if (botStyle == null || botStyle == BotStyle.balanced) return base;
    return StyledAi(base: base, style: botStyle);
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

  BotStyle? _botStyleFromLabel(String value) {
    for (final style in BotStyle.values) {
      if (style.name.toLowerCase() == value.toLowerCase()) return style;
    }
    return null;
  }

  String? _botNameFromOpponentLabel(String label) {
    final marker = RegExp(r'^Practice\s+·\s+(.+)$').firstMatch(label);
    return marker?.group(1);
  }

  void toggleDead(Point p) {
    final cur = _ui;
    if (cur.state.status != GameStatus.scoring) return;
    if (!cur.state.board.inBounds(p) ||
        cur.state.board.cellAt(p) == CellState.empty) {
      return;
    }
    final group = findGroup(cur.state.board, p).stones;
    final next = cur.deadStones.toSet();
    if (cur.deadStones.contains(p)) {
      next.removeAll(group);
    } else {
      next.addAll(group);
    }
    _set(cur.copyWith(deadStones: next));
    _computeScore();
    _autosave();
  }

  void confirmScore() {
    final cur = _ui;
    if (cur.state.status != GameStatus.scoring) return;
    final ended = cur.state.copyWith(status: GameStatus.completed);
    _set(cur.copyWith(
        state: ended, sgf: Sgf.export(cur.state, score: cur.score)));
    _stopClock();
    _archiveAndClear();
  }

  void resumePlay() {
    final cur = _ui;
    if (cur.state.status != GameStatus.scoring) return;
    _generation++;
    _set(cur.copyWith(
      state:
          cur.state.copyWith(status: GameStatus.active, consecutivePasses: 0),
      deadStones: const {},
      score: null,
    ));
    _startClock();
    _autosave();
    _maybeTriggerAi();
  }

  String exportSgf() {
    final cur = _ui;
    final sgf = Sgf.export(cur.state,
        score: cur.score, result: _computeResultLabel(cur));
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
    if (_paused || _ui.state.status != GameStatus.active) return;
    _lastTickMillis = _now;
    _lastSavedMillis = _lastTickMillis;
    _clockTimer =
        Timer.periodic(const Duration(milliseconds: 250), (_) => _tick());
  }

  void _tick() {
    if (_paused || _disposed || _clockTimer == null) return;
    final now = _now;
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
      _generation++;
      _set(_ui.copyWith(aiThinking: false));
      _archiveAndClear();
      _stopClock();
    } else if (now - _lastSavedMillis >= 5000) {
      _lastSavedMillis = now;
      _autosave();
    }
  }

  void pause() {
    if (_paused || _disposed) return;
    _tick();
    _paused = true;
    _generation++;
    _stopClock();
    _set(_ui.copyWith(aiThinking: false));
    _autosave();
  }

  void resume() {
    if (_disposed) return;
    _paused = false;
    _startClock();
    _maybeTriggerAi();
  }

  void setVisible(bool visible) {
    _visible = visible;
    if (visible) {
      resume();
    } else {
      pause();
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      if (_visible) resume();
    } else {
      pause();
    }
  }

  void _stopClock() {
    _clockTimer?.cancel();
    _clockTimer = null;
  }

  @override
  void dispose() {
    pause();
    _disposed = true;
    _generation++;
    WidgetsBinding.instance.removeObserver(this);
    _stopClock();
    super.dispose();
  }
}

class _AiRequest {
  final GoAi ai;
  final GameState state;
  const _AiRequest(this.ai, this.state);
}

MoveIntent _computeMove(_AiRequest request) =>
    request.ai.chooseMove(request.state);
Future<MoveIntent> _runAi(GoAi ai, GameState state) =>
    compute(_computeMove, _AiRequest(ai, state));
