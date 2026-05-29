import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/puzzles/puzzle.dart';
import 'package:go_game/src/domain/puzzles/puzzle_session.dart';

void main() {
  test('every authored puzzle is solvable by its first correct branch', () {
    final raw = File('assets/puzzles/puzzles.json').readAsStringSync();
    final list = (json.decode(raw) as List<dynamic>)
        .map((e) => Puzzle.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
    expect(list, isNotEmpty,
        reason: 'starter puzzle set should have at least one puzzle');
    for (final puzzle in list) {
      final correct = puzzle.solution
          .where((n) => n.outcome == NodeOutcome.correct)
          .toList();
      expect(correct, isNotEmpty,
          reason: 'puzzle ${puzzle.id} (${puzzle.title}) has no correct branch');
      final session = PuzzleSession.start(puzzle);
      final ok = session.play(correct.first.move.point);
      expect(ok, isTrue,
          reason: 'puzzle ${puzzle.id} rejected its own correct move');
      expect(session.status, PuzzleStatus.solved,
          reason: 'puzzle ${puzzle.id} did not reach solved state');
    }
  });
}
