import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';
import 'package:sqflite/sqflite.dart';

import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/scoring.dart';
import '../sgf/sgf.dart';
import 'saved_game.dart';

const _table = 'saved_games';
const _currentId = 'current';

/// Repository over a sqflite database mirroring the original Room schema.
class SavedGameRepo {
  final Database _db;
  final Directory _sgfDir;

  SavedGameRepo._(this._db, this._sgfDir);

  static Future<SavedGameRepo> open() async {
    final base = await getApplicationDocumentsDirectory();
    final path = p.join(base.path, 'go_game.db');
    final db = await openDatabase(
      path,
      version: 4,
      onCreate: (db, _) async {
        await db.execute('''
          CREATE TABLE $_table (
            id TEXT PRIMARY KEY NOT NULL,
            createdAtMillis INTEGER NOT NULL,
            updatedAtMillis INTEGER NOT NULL,
            boardSize INTEGER NOT NULL,
            komi REAL NOT NULL,
            handicap INTEGER NOT NULL,
            ruleset TEXT NOT NULL DEFAULT 'CHINESE',
            status TEXT NOT NULL,
            movesEncoded TEXT NOT NULL,
            opponentLabel TEXT NOT NULL DEFAULT 'Local',
            resultLabel TEXT NOT NULL DEFAULT '',
            youColor TEXT NOT NULL DEFAULT 'BLACK',
            sgfPath TEXT NOT NULL DEFAULT ''
          )
        ''');
      },
      onUpgrade: (db, oldVersion, newVersion) async {
        // Defensive: add columns introduced after v1, ignore failures if already present.
        for (final stmt in const [
          "ALTER TABLE saved_games ADD COLUMN opponentLabel TEXT NOT NULL DEFAULT 'Local'",
          "ALTER TABLE saved_games ADD COLUMN resultLabel TEXT NOT NULL DEFAULT ''",
          "ALTER TABLE saved_games ADD COLUMN youColor TEXT NOT NULL DEFAULT 'BLACK'",
          "ALTER TABLE saved_games ADD COLUMN sgfPath TEXT NOT NULL DEFAULT ''",
          "ALTER TABLE saved_games ADD COLUMN ruleset TEXT NOT NULL DEFAULT 'CHINESE'",
        ]) {
          try {
            await db.execute(stmt);
          } catch (_) {}
        }
      },
    );
    final sgfDir = Directory(p.join(base.path, 'sgf'));
    if (!sgfDir.existsSync()) sgfDir.createSync(recursive: true);
    return SavedGameRepo._(db, sgfDir);
  }

  Future<void> close() => _db.close();

  Future<void> saveCurrent({
    required GameState state,
    required String opponentLabel,
    required StoneColor youColor,
  }) async {
    if (state.status != GameStatus.active &&
        state.status != GameStatus.scoring) {
      return;
    }
    final now = DateTime.now().millisecondsSinceEpoch;
    final existing = await get(_currentId);
    final entity = GameSerializer.toEntity(
      id: _currentId,
      state: state,
      createdAt: existing?.createdAtMillis ?? now,
      updatedAt: now,
      opponentLabel: opponentLabel,
      resultLabel: '',
      youColor: youColor.name.toUpperCase(),
    );
    await _db.insert(_table, entity.toRow(),
        conflictAlgorithm: ConflictAlgorithm.replace);
  }

  Future<GameState?> loadCurrent() async {
    final entity = await get(_currentId);
    return entity == null ? null : GameSerializer.fromEntity(entity);
  }

  Future<void> clearCurrent() async {
    await _db.delete(_table, where: 'id = ?', whereArgs: [_currentId]);
  }

  Future<String> archiveCompleted({
    required GameState state,
    required String opponentLabel,
    required StoneColor youColor,
    required String resultLabel,
    ScoreResult? score,
  }) async {
    final now = DateTime.now().millisecondsSinceEpoch;
    final id = 'game_$now';
    final sgfText = Sgf.export(
      state,
      score: score,
      blackName: youColor == StoneColor.black ? 'You' : opponentLabel,
      whiteName: youColor == StoneColor.white ? 'You' : opponentLabel,
    );
    final f = File(p.join(_sgfDir.path, '$id.sgf'));
    await f.writeAsString(sgfText);
    final entity = GameSerializer.toEntity(
      id: id,
      state: state,
      createdAt: now,
      updatedAt: now,
      opponentLabel: opponentLabel,
      resultLabel: resultLabel,
      youColor: youColor.name.toUpperCase(),
      sgfPath: f.path,
    );
    await _db.insert(_table, entity.toRow(),
        conflictAlgorithm: ConflictAlgorithm.replace);
    return f.path;
  }

  Future<void> delete(String id) async {
    final entity = await get(id);
    if (entity != null && entity.sgfPath.isNotEmpty) {
      final f = File(entity.sgfPath);
      if (f.existsSync()) {
        try {
          await f.delete();
        } catch (_) {}
      }
    }
    await _db.delete(_table, where: 'id = ?', whereArgs: [id]);
  }

  Future<List<SavedGameEntity>> listAll() async {
    final rows = await _db.query(_table, orderBy: 'updatedAtMillis DESC');
    return rows.map(SavedGameEntity.fromRow).toList(growable: false);
  }

  Future<List<SavedGameEntity>> listCompleted({int limit = 10}) async {
    final rows = await _db.query(
      _table,
      where: "status IN ('COMPLETED','RESIGNED')",
      orderBy: 'updatedAtMillis DESC',
      limit: limit,
    );
    return rows.map(SavedGameEntity.fromRow).toList(growable: false);
  }

  Future<SavedGameEntity?> get(String id) async {
    final rows =
        await _db.query(_table, where: 'id = ?', whereArgs: [id], limit: 1);
    if (rows.isEmpty) return null;
    return SavedGameEntity.fromRow(rows.first);
  }
}
