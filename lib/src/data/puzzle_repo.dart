import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show rootBundle;
import 'package:shared_preferences/shared_preferences.dart';

import '../domain/puzzles/puzzle.dart';

const _kAttemptsPrefix = 'puzzle.attempt.';
const _kStreakCurrent = 'puzzle.streak.current';
const _kStreakBest = 'puzzle.streak.best';
const _kStreakLastDate = 'puzzle.streak.lastDate';
const _kDailyDate = 'puzzle.daily.date';
const _kDailyId = 'puzzle.daily.id';
const _kRushBest = 'puzzle.rush.best';

enum AttemptStatus { unsolved, solved, failed }

class PuzzleAttempt {
  final String puzzleId;
  final AttemptStatus status;
  final int mistakes;
  final int hintsUsed;
  final int elapsedMillis;
  final int completedAtMillis;

  const PuzzleAttempt({
    required this.puzzleId,
    required this.status,
    required this.mistakes,
    required this.hintsUsed,
    required this.elapsedMillis,
    required this.completedAtMillis,
  });

  Map<String, dynamic> toJson() => {
        'status': status.name,
        'mistakes': mistakes,
        'hintsUsed': hintsUsed,
        'elapsedMillis': elapsedMillis,
        'completedAtMillis': completedAtMillis,
      };

  static PuzzleAttempt fromJson(String id, Map<String, dynamic> json) =>
      PuzzleAttempt(
        puzzleId: id,
        status: AttemptStatus.values.firstWhere(
          (s) => s.name == json['status'],
          orElse: () => AttemptStatus.unsolved,
        ),
        mistakes: json['mistakes'] as int? ?? 0,
        hintsUsed: json['hintsUsed'] as int? ?? 0,
        elapsedMillis: json['elapsedMillis'] as int? ?? 0,
        completedAtMillis: json['completedAtMillis'] as int? ?? 0,
      );
}

class StreakInfo {
  final int current;
  final int best;
  final DateTime? lastSolvedDate;

  const StreakInfo({
    required this.current,
    required this.best,
    required this.lastSolvedDate,
  });
}

/// Loads puzzles from bundled JSON assets and persists per-puzzle attempt
/// records plus a daily-puzzle streak counter via SharedPreferences.
class PuzzleRepo extends ChangeNotifier {
  static const _assetPath = 'assets/puzzles/puzzles.json';

  final SharedPreferences _prefs;
  List<Puzzle> _puzzles = const [];
  Map<String, PuzzleAttempt> _attempts = const {};

  PuzzleRepo._(this._prefs);

  static Future<PuzzleRepo> load() async {
    final prefs = await SharedPreferences.getInstance();
    final repo = PuzzleRepo._(prefs);
    await repo._loadAll();
    return repo;
  }

  List<Puzzle> get puzzles => _puzzles;
  Map<String, PuzzleAttempt> get attempts => _attempts;

  Puzzle? puzzleById(String id) {
    for (final p in _puzzles) {
      if (p.id == id) return p;
    }
    return null;
  }

  PuzzleAttempt? attemptFor(String puzzleId) => _attempts[puzzleId];

  Future<void> _loadAll() async {
    final raw = await rootBundle.loadString(_assetPath);
    final list = (json.decode(raw) as List<dynamic>)
        .map((e) => Puzzle.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
    _puzzles = list;
    final loaded = <String, PuzzleAttempt>{};
    for (final p in list) {
      final encoded = _prefs.getString('$_kAttemptsPrefix${p.id}');
      if (encoded == null) continue;
      final decoded = json.decode(encoded) as Map<String, dynamic>;
      loaded[p.id] = PuzzleAttempt.fromJson(p.id, decoded);
    }
    _attempts = loaded;
  }

  Future<void> recordAttempt(PuzzleAttempt attempt) async {
    final next = Map<String, PuzzleAttempt>.from(_attempts);
    next[attempt.puzzleId] = attempt;
    _attempts = next;
    await _prefs.setString(
      '$_kAttemptsPrefix${attempt.puzzleId}',
      json.encode(attempt.toJson()),
    );
    notifyListeners();
  }

  /// Pick today's puzzle deterministically and cache it. Calling this multiple
  /// times in the same day returns the same puzzle.
  Puzzle? dailyPuzzle({DateTime? now}) {
    if (_puzzles.isEmpty) return null;
    final today = _dateKey(now ?? DateTime.now());
    final cachedDate = _prefs.getString(_kDailyDate);
    if (cachedDate == today) {
      final id = _prefs.getString(_kDailyId);
      if (id != null) {
        final p = puzzleById(id);
        if (p != null) return p;
      }
    }
    // Pick a deterministic puzzle by hashing the date.
    final idx = today.hashCode.abs() % _puzzles.length;
    final picked = _puzzles[idx];
    unawaited(_prefs.setString(_kDailyDate, today));
    unawaited(_prefs.setString(_kDailyId, picked.id));
    return picked;
  }

  StreakInfo streak() {
    final current = _prefs.getInt(_kStreakCurrent) ?? 0;
    final best = _prefs.getInt(_kStreakBest) ?? 0;
    final lastIso = _prefs.getString(_kStreakLastDate);
    final lastDate = lastIso == null ? null : DateTime.tryParse(lastIso);
    return StreakInfo(current: current, best: best, lastSolvedDate: lastDate);
  }

  /// Update the daily-puzzle streak. Call after a daily puzzle is solved.
  Future<StreakInfo> markDailySolved({DateTime? now}) async {
    final today = (now ?? DateTime.now());
    final todayKey = _dateKey(today);
    final lastIso = _prefs.getString(_kStreakLastDate);
    final lastDate = lastIso == null ? null : DateTime.tryParse(lastIso);
    var current = _prefs.getInt(_kStreakCurrent) ?? 0;
    final best = _prefs.getInt(_kStreakBest) ?? 0;

    if (lastDate != null && _dateKey(lastDate) == todayKey) {
      // Already counted today.
      return streak();
    }
    if (lastDate != null && _isYesterday(lastDate, today)) {
      current += 1;
    } else {
      current = 1;
    }
    await _prefs.setInt(_kStreakCurrent, current);
    await _prefs.setString(_kStreakLastDate, today.toIso8601String());
    if (current > best) {
      await _prefs.setInt(_kStreakBest, current);
    }
    notifyListeners();
    return streak();
  }

  int rushBest() => _prefs.getInt(_kRushBest) ?? 0;

  Future<void> recordRushScore(int score) async {
    final current = rushBest();
    if (score > current) {
      await _prefs.setInt(_kRushBest, score);
      notifyListeners();
    }
  }

  static String _dateKey(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

  static bool _isYesterday(DateTime then, DateTime now) {
    final y = DateTime(now.year, now.month, now.day - 1);
    return then.year == y.year && then.month == y.month && then.day == y.day;
  }
}
