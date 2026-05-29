import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show rootBundle;
import 'package:shared_preferences/shared_preferences.dart';

import '../domain/drills/drill.dart';

const _kWinsPrefix = 'drill.wins.';
const _kMasteredPrefix = 'drill.mastered.';

/// Tracks per-drill consecutive-win counters used to gate mastery.
class DrillRepo extends ChangeNotifier {
  static const _assetPath = 'assets/drills/drills.json';

  final SharedPreferences _prefs;
  List<Drill> _drills = const [];

  DrillRepo._(this._prefs);

  static Future<DrillRepo> load() async {
    final prefs = await SharedPreferences.getInstance();
    final repo = DrillRepo._(prefs);
    await repo._loadAll();
    return repo;
  }

  Future<void> _loadAll() async {
    final raw = await rootBundle.loadString(_assetPath);
    final list = (json.decode(raw) as List<dynamic>)
        .map((e) => Drill.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
    _drills = list;
  }

  List<Drill> get drills => _drills;

  Drill? byId(String id) {
    for (final d in _drills) {
      if (d.id == id) return d;
    }
    return null;
  }

  int winsFor(String drillId) => _prefs.getInt('$_kWinsPrefix$drillId') ?? 0;

  bool isMastered(String drillId) =>
      _prefs.getBool('$_kMasteredPrefix$drillId') ?? false;

  /// Record a successful attempt; returns the updated streak.
  Future<int> recordWin(Drill drill) async {
    final wins = winsFor(drill.id) + 1;
    await _prefs.setInt('$_kWinsPrefix${drill.id}', wins);
    if (wins >= drill.requiredWins) {
      await _prefs.setBool('$_kMasteredPrefix${drill.id}', true);
    }
    notifyListeners();
    return wins;
  }

  /// Reset the streak on a loss.
  Future<void> recordLoss(Drill drill) async {
    await _prefs.setInt('$_kWinsPrefix${drill.id}', 0);
    notifyListeners();
  }
}
