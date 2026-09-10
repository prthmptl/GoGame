import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/data/saved_game.dart';
import 'package:go_game/src/data/saved_game_repo.dart';
import 'package:go_game/src/domain/clock/clock_controller.dart';
import 'package:go_game/src/domain/clock/time_control.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/ui/screens/game_view_model.dart';

void main() {
  test('corrupt saved clocks fail resume without throwing', () async {
    final repo = _MemoryRepo();
    final row = GameSerializer.toEntity(
            id: 'current',
            state: GameState.newGame(const GameConfig(boardSize: 9)),
            createdAt: 0,
            updatedAt: 0)
        .toRow();
    row['runtimeJson'] = '{"black":{"mainMillis":"broken"}}';
    repo.current = SavedGameEntity.fromRow(row);
    final vm = GameViewModel(repo: repo);
    expect(await vm.resumeCurrent(), isFalse);
    expect(vm.ui.rejection, contains('could not be read'));
    vm.dispose();
    await vm.saved;
  });
  TestWidgetsFlutterBinding.ensureInitialized();
  const config = GameConfig(boardSize: 9);

  test('stale AI work cannot play into a new game or disposed model', () async {
    final result = Completer<MoveIntent>();
    final vm = GameViewModel(chooseMove: (_, __) => result.future);
    vm.startGame(
        config: config,
        opponent: Opponent.ai,
        aiPlays: StoneColor.black,
        timeControl: const TimeControl.none());
    expect(vm.ui.aiThinking, isTrue);
    vm.startGame(
        config: config,
        opponent: Opponent.human,
        timeControl: const TimeControl.none());
    result.complete(const MoveIntent.place(Point(4, 4)));
    await Future<void>.delayed(Duration.zero);
    expect(vm.ui.state.history, isEmpty);
    expect(vm.ui.aiThinking, isFalse);
    vm.dispose();

    final pending = Completer<MoveIntent>();
    final disposed = GameViewModel(chooseMove: (_, __) => pending.future);
    disposed.startGame(
        config: config,
        opponent: Opponent.ai,
        aiPlays: StoneColor.black,
        timeControl: const TimeControl.none());
    disposed.dispose();
    pending.complete(const MoveIntent.pass());
    await Future<void>.delayed(Duration.zero);
  });

  test('undo invalidates pending AI and restores the player turn', () async {
    final pending = Completer<MoveIntent>();
    final vm = GameViewModel(chooseMove: (_, __) => pending.future);
    vm.startGame(
        config: config,
        opponent: Opponent.ai,
        timeControl: const TimeControl.none());
    vm.tap(const Point(4, 4));
    vm.undo();
    pending.complete(const MoveIntent.place(Point(0, 0)));
    await Future<void>.delayed(Duration.zero);
    expect(vm.ui.state.history, isEmpty);
    expect(vm.ui.state.currentPlayer, StoneColor.black);
    expect(vm.ui.aiThinking, isFalse);
    vm.dispose();
  });

  test('pass cannot impersonate AI and resign always resigns the human',
      () async {
    final pending = Completer<MoveIntent>();
    final vm = GameViewModel(chooseMove: (_, __) => pending.future);
    vm.startGame(
        config: config,
        opponent: Opponent.ai,
        timeControl: const TimeControl.none());
    vm.tap(const Point(4, 4));
    vm.pass();
    expect(vm.ui.state.history.length, 1);
    vm.resign();
    expect(vm.ui.state.history.last.player, StoneColor.black);
    expect(vm.exportSgf(), contains('RE[W+R]'));
    pending.complete(const MoveIntent.pass());
    await Future<void>.delayed(Duration.zero);
    expect(vm.ui.state.history.length, 2);
    vm.dispose();
  });

  test('moves charge the mover before handing over the clock', () {
    var now = 0;
    final vm = GameViewModel(nowMillis: () => now);
    vm.startGame(
        config: config,
        opponent: Opponent.human,
        timeControl: const TimeControl.absolute(mainSeconds: 1));
    now = 900;
    vm.tap(const Point(0, 0));
    expect(vm.ui.blackClock.mainMillis, 100);
    expect(vm.ui.whiteClock.mainMillis, 1000);
    now = 1901;
    vm.tap(const Point(1, 1));
    expect(vm.ui.timeoutLoser, StoneColor.white);
    expect(vm.ui.state.history.length, 1);
    expect(vm.exportSgf(), contains('RE[B+T]'));
    vm.dispose();
  });

  test('scoring toggles full groups and cannot finish active games', () {
    final vm = GameViewModel();
    vm.startGame(
        config: config,
        opponent: Opponent.human,
        timeControl: const TimeControl.none());
    vm.confirmScore();
    expect(vm.ui.state.status, GameStatus.active);
    vm.tap(const Point(0, 0));
    vm.tap(const Point(8, 8));
    vm.tap(const Point(0, 1));
    vm.pass();
    vm.pass();
    vm.toggleDead(const Point(0, 0));
    expect(vm.ui.deadStones, {const Point(0, 0), const Point(0, 1)});
    vm.toggleDead(const Point(0, 1));
    expect(vm.ui.deadStones, isEmpty);
    vm.toggleDead(const Point(-1, 0));
    vm.resumePlay();
    vm.tap(const Point(7, 7));
    final restored = GameSerializer.fromEntity(GameSerializer.toEntity(
        id: 'test', state: vm.ui.state, createdAt: 0, updatedAt: 0));
    expect(restored.history.length, vm.ui.state.history.length);
    expect(restored.status, GameStatus.active);
    vm.dispose();
  });

  test(
      'resume restores remaining time and new games cannot be erased by old archive',
      () async {
    final repo = _MemoryRepo();
    var now = 0;
    final vm = GameViewModel(repo: repo, nowMillis: () => now);
    vm.startGame(
        config: config,
        opponent: Opponent.human,
        timeControl: const TimeControl.absolute(mainSeconds: 60));
    await vm.saved;
    expect(repo.current, isNotNull); // Includes a game with zero moves.
    now = 2500;
    vm.tap(const Point(0, 0));
    await vm.saved;
    vm.pause();
    expect(await vm.resumeCurrent(), isTrue);
    expect(vm.ui.blackClock.mainMillis, 57500);
    vm.resign();
    vm.startGame(
        config: config,
        opponent: Opponent.human,
        timeControl: const TimeControl.none());
    await vm.saved;
    expect(repo.archives, 1);
    expect(repo.current?.status, 'ACTIVE');
    expect(repo.current?.movesEncoded, '');
    vm.dispose();
    await vm.saved;
  });
}

class _MemoryRepo implements SavedGameRepo {
  SavedGameEntity? current;
  int archives = 0;
  @override
  Future<void> clearCurrent() async {
    current = null;
  }

  @override
  Future<SavedGameEntity?> loadCurrentEntity() async => current;
  @override
  Future<void> saveCurrent(
      {required GameState state,
      required String opponentLabel,
      required StoneColor youColor,
      TimeControl? timeControl,
      String? botName,
      String? botStyle,
      AiDifficulty aiDifficulty = AiDifficulty.beginner,
      ClockSnapshot? blackClock,
      ClockSnapshot? whiteClock,
      Set<Point> deadStones = const {}}) async {
    await Future<void>.delayed(Duration.zero);
    current = GameSerializer.toEntity(
        id: 'current',
        state: state,
        createdAt: 0,
        updatedAt: 0,
        opponentLabel: opponentLabel,
        youColor: youColor.name.toUpperCase(),
        timeControl: timeControl,
        blackClock: blackClock,
        whiteClock: whiteClock,
        deadStones: deadStones);
  }

  @override
  Future<String> archiveCompleted(
      {required GameState state,
      required String opponentLabel,
      required StoneColor youColor,
      required String resultLabel,
      dynamic score,
      TimeControl? timeControl,
      String? botName,
      String? botStyle,
      AiDifficulty aiDifficulty = AiDifficulty.beginner}) async {
    await Future<void>.delayed(Duration.zero);
    archives++;
    current = null;
    return '';
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
