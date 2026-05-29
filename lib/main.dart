import 'package:flutter/material.dart';

import 'src/app.dart';
import 'src/data/drill_repo.dart';
import 'src/data/lesson_repo.dart';
import 'src/data/profile_store.dart';
import 'src/data/puzzle_repo.dart';
import 'src/data/saved_game_repo.dart';
import 'src/data/settings_store.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final repo = await SavedGameRepo.open();
  final settings = await SettingsStore.load();
  final puzzles = await PuzzleRepo.load();
  final drills = await DrillRepo.load();
  final lessons = await LessonRepo.load();
  final profile = await ProfileStore.load();
  runApp(GoApp(
    repo: repo,
    settings: settings,
    puzzles: puzzles,
    drills: drills,
    lessons: lessons,
    profile: profile,
  ));
}
