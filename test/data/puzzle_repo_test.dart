import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:go_game/src/data/puzzle_repo.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('missed days reset displayed streak and repeated completion counts once',
      () async {
    SharedPreferences.setMockInitialValues({});
    final repo = await PuzzleRepo.load();
    final day = DateTime(2026, 9, 1);
    expect((await repo.markDailySolved(now: day)).current, 1);
    expect((await repo.markDailySolved(now: day)).current, 1);
    expect((await repo.markDailySolved(now: DateTime(2026, 9, 2))).current, 2);
    expect(repo.streak(now: DateTime(2026, 9, 4)).current, 0);
    expect(repo.streak(now: DateTime(2026, 9, 4)).best, 2);
    repo.dispose();
  });
  test('daily selection is stable and a corrupt attempt does not block startup',
      () async {
    SharedPreferences.setMockInitialValues({});
    final first = await PuzzleRepo.load();
    final id = first.puzzles.first.id;
    final expected = first.dailyPuzzle(now: DateTime(2026, 9, 10))!.id;
    first.dispose();
    SharedPreferences.setMockInitialValues(
        {'puzzle.attempt.$id': 'broken json'});
    final second = await PuzzleRepo.load();
    expect(second.puzzles, isNotEmpty);
    expect(second.attemptFor(id), isNull);
    expect(second.dailyPuzzle(now: DateTime(2026, 9, 10))!.id, expected);
    second.dispose();
  });
}
