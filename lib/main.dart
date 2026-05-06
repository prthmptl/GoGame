import 'package:flutter/material.dart';

import 'src/app.dart';
import 'src/data/saved_game_repo.dart';
import 'src/data/settings_store.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final repo = await SavedGameRepo.open();
  final settings = await SettingsStore.load();
  runApp(WeiqiApp(repo: repo, settings: settings));
}
