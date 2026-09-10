import '../models.dart';
import 'time_control.dart';

/// Snapshot of a single player's clock at one instant.
class ClockSnapshot {
  /// Remaining main time in milliseconds.
  final int mainMillis;

  /// Remaining time inside the current overtime period, in milliseconds.
  /// Meaningful for byo-yomi and Canadian once [mainMillis] reaches zero.
  final int periodMillis;

  /// Byo-yomi periods remaining (including the active one). 0 means flagged.
  final int periodsLeft;

  /// Stones remaining in the current Canadian period.
  final int stonesLeftInPeriod;

  /// True once the player is playing inside an overtime period.
  final bool inOvertime;

  /// True if the player has run out of time entirely.
  final bool flagged;

  const ClockSnapshot({
    required this.mainMillis,
    required this.periodMillis,
    required this.periodsLeft,
    required this.stonesLeftInPeriod,
    required this.inOvertime,
    required this.flagged,
  });

  Map<String, Object> toJson() => {
        'mainMillis': mainMillis,
        'periodMillis': periodMillis,
        'periodsLeft': periodsLeft,
        'stonesLeftInPeriod': stonesLeftInPeriod,
        'inOvertime': inOvertime,
        'flagged': flagged,
      };

  factory ClockSnapshot.fromJson(Map<String, dynamic> json) => ClockSnapshot(
        mainMillis: json['mainMillis'] as int,
        periodMillis: json['periodMillis'] as int,
        periodsLeft: json['periodsLeft'] as int,
        stonesLeftInPeriod: json['stonesLeftInPeriod'] as int,
        inOvertime: json['inOvertime'] as bool,
        flagged: json['flagged'] as bool,
      );

  ClockSnapshot copyWith({
    int? mainMillis,
    int? periodMillis,
    int? periodsLeft,
    int? stonesLeftInPeriod,
    bool? inOvertime,
    bool? flagged,
  }) =>
      ClockSnapshot(
        mainMillis: mainMillis ?? this.mainMillis,
        periodMillis: periodMillis ?? this.periodMillis,
        periodsLeft: periodsLeft ?? this.periodsLeft,
        stonesLeftInPeriod: stonesLeftInPeriod ?? this.stonesLeftInPeriod,
        inOvertime: inOvertime ?? this.inOvertime,
        flagged: flagged ?? this.flagged,
      );

  /// User-facing display string, e.g. "08:42" or "00:25 (3)" for byo-yomi.
  String formatted(TimeControl tc) {
    if (tc.kind == TimeControlKind.none) return '∞';
    if (flagged) return '00:00';
    if (!inOvertime) return _hms(mainMillis);
    switch (tc.kind) {
      case TimeControlKind.byoYomi:
        return '${_hms(periodMillis)} ($periodsLeft)';
      case TimeControlKind.canadian:
        return '${_hms(periodMillis)}/$stonesLeftInPeriod';
      default:
        return _hms(periodMillis);
    }
  }

  static String _hms(int millis) {
    if (millis < 0) millis = 0;
    final total = millis ~/ 1000;
    final m = total ~/ 60;
    final s = total % 60;
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }
}

/// Pure-logic clock controller. Holds per-player snapshots and applies
/// time deltas + move events. UI layers handle the wall clock; this class
/// just transitions state based on elapsed milliseconds.
class ClockController {
  final TimeControl control;
  ClockSnapshot _black;
  ClockSnapshot _white;
  StoneColor _active;

  ClockController(this.control, {StoneColor active = StoneColor.black})
      : _black = _seedFor(control),
        _white = _seedFor(control),
        _active = active;

  ClockController.restore(
    this.control, {
    required ClockSnapshot black,
    required ClockSnapshot white,
    required StoneColor active,
  })  : _black = black,
        _white = white,
        _active = active;

  static ClockSnapshot _seedFor(TimeControl tc) {
    if (tc.kind == TimeControlKind.none) {
      return const ClockSnapshot(
        mainMillis: 0,
        periodMillis: 0,
        periodsLeft: 0,
        stonesLeftInPeriod: 0,
        inOvertime: false,
        flagged: false,
      );
    }
    return ClockSnapshot(
      mainMillis: tc.mainSeconds * 1000,
      periodMillis: tc.periodSeconds * 1000,
      periodsLeft: tc.periods,
      stonesLeftInPeriod: tc.stonesPerPeriod,
      inOvertime: false,
      flagged: false,
    );
  }

  ClockSnapshot get black => _black;
  ClockSnapshot get white => _white;
  StoneColor get active => _active;

  /// Returns the [StoneColor] whose clock just flagged, or `null`.
  StoneColor? tick(int elapsedMillis) {
    if (control.kind == TimeControlKind.none) return null;
    if (elapsedMillis <= 0) return null;
    final updated = _applyTick(_get(_active), elapsedMillis);
    _set(_active, updated);
    return updated.flagged ? _active : null;
  }

  /// Note an accepted move by [mover]; apply increments / period resets and
  /// switch the active clock to the opponent.
  void onMovePlayed(StoneColor mover) {
    final current = _get(mover);
    final next = _applyMoveBonus(current);
    _set(mover, next);
    _active = mover.other;
  }

  void switchActive(StoneColor active) {
    _active = active;
  }

  ClockSnapshot _get(StoneColor c) => c == StoneColor.black ? _black : _white;

  void _set(StoneColor c, ClockSnapshot snap) {
    if (c == StoneColor.black) {
      _black = snap;
    } else {
      _white = snap;
    }
  }

  ClockSnapshot _applyTick(ClockSnapshot s, int elapsedMillis) {
    if (s.flagged) return s;
    if (!s.inOvertime) {
      final main = s.mainMillis - elapsedMillis;
      if (main > 0) {
        return s.copyWith(mainMillis: main);
      }
      // Main time exhausted; spend the overflow inside the first period.
      final overflow = -main;
      switch (control.kind) {
        case TimeControlKind.absolute:
        case TimeControlKind.fischer:
        case TimeControlKind.none:
          return s.copyWith(mainMillis: 0, flagged: true);
        case TimeControlKind.byoYomi:
        case TimeControlKind.canadian:
          return _consumeOvertime(
              s.copyWith(mainMillis: 0, inOvertime: true), overflow);
      }
    }
    return _consumeOvertime(s, elapsedMillis);
  }

  ClockSnapshot _consumeOvertime(ClockSnapshot s, int elapsedMillis) {
    var remaining = s.periodMillis - elapsedMillis;
    var periodsLeft = s.periodsLeft;
    var stonesLeft = s.stonesLeftInPeriod;
    while (remaining <= 0) {
      switch (control.kind) {
        case TimeControlKind.byoYomi:
          periodsLeft -= 1;
          if (periodsLeft <= 0) {
            return s.copyWith(
              periodMillis: 0,
              periodsLeft: 0,
              flagged: true,
            );
          }
          // Roll the overflow into the next fresh period.
          remaining += control.periodSeconds * 1000;
          break;
        case TimeControlKind.canadian:
          // Running out the Canadian period without playing the required
          // stones flags the player.
          return s.copyWith(
            periodMillis: 0,
            stonesLeftInPeriod: stonesLeft,
            flagged: true,
          );
        default:
          return s.copyWith(periodMillis: 0, flagged: true);
      }
    }
    return s.copyWith(
      periodMillis: remaining,
      periodsLeft: periodsLeft,
      stonesLeftInPeriod: stonesLeft,
    );
  }

  ClockSnapshot _applyMoveBonus(ClockSnapshot s) {
    if (control.kind == TimeControlKind.none || s.flagged) return s;
    switch (control.kind) {
      case TimeControlKind.absolute:
      case TimeControlKind.none:
        return s;
      case TimeControlKind.fischer:
        return s.copyWith(
            mainMillis: s.mainMillis + control.incrementSeconds * 1000);
      case TimeControlKind.byoYomi:
        if (!s.inOvertime) return s;
        // Completing a move during byo-yomi resets the period clock.
        return s.copyWith(periodMillis: control.periodSeconds * 1000);
      case TimeControlKind.canadian:
        if (!s.inOvertime) return s;
        final stonesLeft = s.stonesLeftInPeriod - 1;
        if (stonesLeft <= 0) {
          return s.copyWith(
            stonesLeftInPeriod: control.stonesPerPeriod,
            periodMillis: control.periodSeconds * 1000,
          );
        }
        return s.copyWith(stonesLeftInPeriod: stonesLeft);
    }
  }
}
