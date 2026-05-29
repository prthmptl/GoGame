import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/clock/clock_controller.dart';
import 'package:go_game/src/domain/clock/time_control.dart';
import 'package:go_game/src/domain/models.dart';

void main() {
  group('Absolute', () {
    test('main time drains and flags when zero', () {
      final c = ClockController(const TimeControl.absolute(mainSeconds: 2));
      expect(c.tick(1500), isNull);
      expect(c.black.mainMillis, 500);
      final flagged = c.tick(600);
      expect(flagged, StoneColor.black);
      expect(c.black.flagged, isTrue);
    });

    test('only the active player loses time', () {
      final c = ClockController(const TimeControl.absolute(mainSeconds: 5));
      c.tick(1000);
      c.switchActive(StoneColor.white);
      c.tick(1000);
      expect(c.black.mainMillis, 4000);
      expect(c.white.mainMillis, 4000);
    });
  });

  group('Fischer', () {
    test('increment is added on each completed move', () {
      final c = ClockController(
          const TimeControl.fischer(mainSeconds: 10, incrementSeconds: 3));
      c.tick(2000);
      expect(c.black.mainMillis, 8000);
      c.onMovePlayed(StoneColor.black);
      expect(c.black.mainMillis, 11000);
      expect(c.active, StoneColor.white);
    });
  });

  group('Byo-yomi', () {
    test('moves into overtime when main time exhausted', () {
      final c = ClockController(const TimeControl.byoYomi(
          mainSeconds: 1, periods: 3, periodSeconds: 30));
      c.tick(1500);
      expect(c.black.inOvertime, isTrue);
      expect(c.black.flagged, isFalse);
      expect(c.black.periodMillis, lessThan(30000));
      expect(c.black.periodsLeft, 3);
    });

    test('completing a move during overtime resets the period clock', () {
      final c = ClockController(const TimeControl.byoYomi(
          mainSeconds: 0, periods: 3, periodSeconds: 30));
      // Forces immediate overtime.
      c.tick(10000);
      expect(c.black.inOvertime, isTrue);
      expect(c.black.periodMillis, 20000);
      c.onMovePlayed(StoneColor.black);
      expect(c.black.periodMillis, 30000);
      expect(c.black.periodsLeft, 3);
    });

    test('consumes successive periods then flags', () {
      final c = ClockController(const TimeControl.byoYomi(
          mainSeconds: 0, periods: 2, periodSeconds: 5));
      c.tick(7000);
      expect(c.black.inOvertime, isTrue);
      expect(c.black.periodsLeft, 1);
      final flagged = c.tick(6000);
      expect(flagged, StoneColor.black);
      expect(c.black.flagged, isTrue);
    });
  });

  group('Canadian', () {
    test('plays stones in the period to reset the clock', () {
      final c = ClockController(const TimeControl.canadian(
          mainSeconds: 0, stonesPerPeriod: 2, periodSeconds: 60));
      c.tick(10000);
      expect(c.black.inOvertime, isTrue);
      expect(c.black.periodMillis, 50000);
      c.onMovePlayed(StoneColor.black);
      expect(c.black.stonesLeftInPeriod, 1);
      c.switchActive(StoneColor.black);
      c.onMovePlayed(StoneColor.black);
      // The second stone in the period resets period clock + stone counter.
      expect(c.black.stonesLeftInPeriod, 2);
      expect(c.black.periodMillis, 60000);
    });

    test('running out of the period flags the player', () {
      final c = ClockController(const TimeControl.canadian(
          mainSeconds: 0, stonesPerPeriod: 5, periodSeconds: 2));
      final flagged = c.tick(3000);
      expect(flagged, StoneColor.black);
      expect(c.black.flagged, isTrue);
    });
  });

  test('none kind never ticks', () {
    final c = ClockController(const TimeControl.none());
    final flagged = c.tick(60 * 60 * 1000);
    expect(flagged, isNull);
    expect(c.black.flagged, isFalse);
  });
}
