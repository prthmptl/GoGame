import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum BoardThemeKind { classicWood, minimalPaper, darkSlate, highContrast }

class AppSettings {
  final BoardThemeKind boardTheme;
  final bool showCoordinates;
  final bool showMoveNumbers;
  final bool beginnerHints;

  const AppSettings({
    this.boardTheme = BoardThemeKind.classicWood,
    this.showCoordinates = false,
    this.showMoveNumbers = false,
    this.beginnerHints = true,
  });

  AppSettings copyWith({
    BoardThemeKind? boardTheme,
    bool? showCoordinates,
    bool? showMoveNumbers,
    bool? beginnerHints,
  }) =>
      AppSettings(
        boardTheme: boardTheme ?? this.boardTheme,
        showCoordinates: showCoordinates ?? this.showCoordinates,
        showMoveNumbers: showMoveNumbers ?? this.showMoveNumbers,
        beginnerHints: beginnerHints ?? this.beginnerHints,
      );
}

/// Persistent settings backed by shared_preferences. Notifies listeners on update.
class SettingsStore extends ChangeNotifier {
  static const _kTheme = 'theme';
  static const _kCoords = 'coords';
  static const _kMoveNumbers = 'moveNumbers';
  static const _kHints = 'hints';

  final SharedPreferences _prefs;
  AppSettings _value;

  SettingsStore._(this._prefs) : _value = _load(_prefs);

  AppSettings get value => _value;

  static Future<SettingsStore> load() async {
    final prefs = await SharedPreferences.getInstance();
    return SettingsStore._(prefs);
  }

  static AppSettings _load(SharedPreferences sp) {
    final themeName = sp.getString(_kTheme) ?? BoardThemeKind.classicWood.name;
    final theme = BoardThemeKind.values.firstWhere(
      (e) => e.name == themeName,
      orElse: () => BoardThemeKind.classicWood,
    );
    return AppSettings(
      boardTheme: theme,
      showCoordinates: sp.getBool(_kCoords) ?? false,
      showMoveNumbers: sp.getBool(_kMoveNumbers) ?? false,
      beginnerHints: sp.getBool(_kHints) ?? true,
    );
  }

  Future<void> update(AppSettings Function(AppSettings) transform) async {
    final next = transform(_value);
    await _prefs.setString(_kTheme, next.boardTheme.name);
    await _prefs.setBool(_kCoords, next.showCoordinates);
    await _prefs.setBool(_kMoveNumbers, next.showMoveNumbers);
    await _prefs.setBool(_kHints, next.beginnerHints);
    _value = next;
    notifyListeners();
  }
}
