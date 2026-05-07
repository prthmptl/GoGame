import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import '../../data/saved_game_repo.dart';
import '../../domain/ai/beginner_ai.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/rules.dart';
import '../../domain/scoring.dart';
import '../../sgf/sgf.dart';

enum Opponent { human, ai }

const _defaultMainMillis = 10 * 60 * 1000;

class GameUi {
  final GameState state;
  final String? rejection;
  final Set<Point> deadStones;
  final ScoreResult? score;
  final bool aiThinking;
  final Opponent opponent;
  final StoneColor aiPlays;
  final String? sgf;
  final int blackMillis;
  final int whiteMillis;
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
    this.sgf,
    this.blackMillis = _defaultMainMillis,
    this.whiteMillis = _defaultMainMillis,
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
    Object? sgf = _sentinel,
    int? blackMillis,
    int? whiteMillis,
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
        sgf: identical(sgf, _sentinel) ? this.sgf : sgf as String?,
        blackMillis: blackMillis ?? this.blackMillis,
        whiteMillis: whiteMillis ?? this.whiteMillis,
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
  final BeginnerAi _ai = BeginnerAi();

  GameUi _ui = GameUi(state: GameState.newGame(const GameConfig(boardSize: 9)));
  Timer? _clockTimer;
  int _lastTickMillis = 0;

  GameViewModel({this.repo});

  GameUi get ui => _ui;

  void _set(GameUi next) {
    _ui = next;
    notifyListeners();
  }

  String _opponentLabel(GameUi ui) =>
      ui.opponent == Opponent.ai ? 'AI' : 'Local';

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
    bool showHints = false,
  }) {
    _set(GameUi(
      state: GameState.newGame(config),
      opponent: opponent,
      aiPlays: aiPlays,
      blackMillis: _defaultMainMillis,
      whiteMillis: _defaultMainMillis,
      showHints: showHints,
    ));
    _startClock();
    _maybeTriggerAi();
  }

  void loadGame(GameState state, {Opponent opponent = Opponent.human}) {
    _set(GameUi(state: state, opponent: opponent));
    _startClock();
  }

  Future<bool> resumeCurrent() async {
    final state = await repo?.loadCurrent();
    if (state == null) return false;
    if (state.status == GameStatus.completed ||
        state.status == GameStatus.resigned) {
      return false;
    }
    _set(GameUi(state: state, opponent: Opponent.human));
    _startClock();
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
    final isAiMove = cur.opponent == Opponent.ai &&
        cur.state.currentPlayer == cur.aiPlays;
    if (intent.type == MoveType.placeStone && !isAiMove) {
      final prevCaps = cur.state.capturesByBlack + cur.state.capturesByWhite;
      final newCaps = next.capturesByBlack + next.capturesByWhite;
      if (newCaps > prevCaps) {
        HapticFeedback.mediumImpact();
      } else {
        HapticFeedback.lightImpact();
      }
    }
    _set(cur.copyWith(state: next, rejection: null, pendingPoint: null));
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
    final active = cur.state.currentPlayer;
    final b = active == StoneColor.black
        ? math.max(0, cur.blackMillis - delta)
        : cur.blackMillis;
    final w = active == StoneColor.white
        ? math.max(0, cur.whiteMillis - delta)
        : cur.whiteMillis;
    StoneColor? timeoutLoser = cur.timeoutLoser;
    if (b == 0 && timeoutLoser == null) timeoutLoser = StoneColor.black;
    if (w == 0 && timeoutLoser == null) timeoutLoser = StoneColor.white;
    final newStatus =
        timeoutLoser != null ? GameStatus.completed : cur.state.status;
    _set(cur.copyWith(
      blackMillis: b,
      whiteMillis: w,
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

  static String formatTime(int millis) {
    final total = math.max(0, millis ~/ 1000);
    final m = total ~/ 60;
    final s = total % 60;
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }
}
