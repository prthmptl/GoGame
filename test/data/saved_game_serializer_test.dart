import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/data/saved_game.dart';
import 'package:go_game/src/domain/clock/time_control.dart';
import 'package:go_game/src/domain/game_state.dart';
import 'package:go_game/src/domain/models.dart';

void main() {
  test('serializes game variant, bot metadata, and time control', () {
    final state = GameState.newGame(const GameConfig(
      boardSize: 9,
      variant: GameVariant.atariGo,
    ));
    final entity = GameSerializer.toEntity(
      id: 'current',
      state: state,
      createdAt: 1,
      updatedAt: 2,
      opponentLabel: 'Practice · Kiri',
      youColor: 'BLACK',
      timeControl: const TimeControl.byoYomi(
        mainSeconds: 600,
        periods: 3,
        periodSeconds: 30,
      ),
      botName: 'Kiri',
      botStyle: 'calm',
      aiDifficulty: AiDifficulty.beginner,
    );

    expect(entity.gameVariant, GameVariant.atariGo.name);
    expect(entity.botName, 'Kiri');
    expect(entity.botStyle, 'calm');
    expect(entity.aiDifficulty, AiDifficulty.beginner.name);
    expect(entity.timeControlKind, TimeControlKind.byoYomi.name);
    expect(entity.timeMainSeconds, 600);
    expect(entity.timePeriods, 3);
    expect(entity.timePeriodSeconds, 30);
  });

  test('restores game variant and time control from saved entity', () {
    final state = GameState.newGame(const GameConfig(
      boardSize: 9,
      variant: GameVariant.atariGo,
    ));
    final entity = GameSerializer.toEntity(
      id: 'current',
      state: state,
      createdAt: 1,
      updatedAt: 2,
      opponentLabel: 'Practice · Mei',
      youColor: 'WHITE',
      timeControl:
          const TimeControl.fischer(mainSeconds: 300, incrementSeconds: 3),
      botName: 'Mei',
      botStyle: 'aggressive',
      aiDifficulty: AiDifficulty.beginner,
    );

    final restored = GameSerializer.fromEntity(entity);
    final restoredClock = GameSerializer.timeControlFromEntity(entity);

    expect(restored.config.variant, GameVariant.atariGo);
    expect(restoredClock.kind, TimeControlKind.fischer);
    expect(restoredClock.mainSeconds, 300);
    expect(restoredClock.incrementSeconds, 3);
  });
}
