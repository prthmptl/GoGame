enum TimeControlKind { none, absolute, fischer, byoYomi, canadian }

extension TimeControlKindLabel on TimeControlKind {
  String get label => switch (this) {
        TimeControlKind.none => 'None',
        TimeControlKind.absolute => 'Absolute',
        TimeControlKind.fischer => 'Fischer',
        TimeControlKind.byoYomi => 'Byo-yomi',
        TimeControlKind.canadian => 'Canadian',
      };
}

class TimeControl {
  final TimeControlKind kind;
  final int mainSeconds;
  final int incrementSeconds;
  final int periodSeconds;
  final int periods;
  final int stonesPerPeriod;

  const TimeControl.none()
      : kind = TimeControlKind.none,
        mainSeconds = 0,
        incrementSeconds = 0,
        periodSeconds = 0,
        periods = 0,
        stonesPerPeriod = 0;

  const TimeControl.absolute({required this.mainSeconds})
      : kind = TimeControlKind.absolute,
        incrementSeconds = 0,
        periodSeconds = 0,
        periods = 0,
        stonesPerPeriod = 0;

  const TimeControl.fischer({
    required this.mainSeconds,
    required this.incrementSeconds,
  })  : kind = TimeControlKind.fischer,
        periodSeconds = 0,
        periods = 0,
        stonesPerPeriod = 0;

  const TimeControl.byoYomi({
    required this.mainSeconds,
    required this.periods,
    required this.periodSeconds,
  })  : kind = TimeControlKind.byoYomi,
        incrementSeconds = 0,
        stonesPerPeriod = 0;

  const TimeControl.canadian({
    required this.mainSeconds,
    required this.stonesPerPeriod,
    required this.periodSeconds,
  })  : kind = TimeControlKind.canadian,
        incrementSeconds = 0,
        periods = 0;

  /// Short user-facing summary, e.g. "10 min + 5 sec" or "10 min · 5×30s byo-yomi".
  String describe() {
    String mainPart() {
      final m = mainSeconds ~/ 60;
      final s = mainSeconds % 60;
      if (s == 0) return '$m min';
      return '${m}m ${s}s';
    }

    return switch (kind) {
      TimeControlKind.none => 'No clock',
      TimeControlKind.absolute => mainPart(),
      TimeControlKind.fischer => '${mainPart()} + ${incrementSeconds}s',
      TimeControlKind.byoYomi =>
        '${mainPart()} · $periods × ${periodSeconds}s byo-yomi',
      TimeControlKind.canadian =>
        '${mainPart()} · $stonesPerPeriod/${periodSeconds}s Canadian',
    };
  }
}
