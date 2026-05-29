import 'package:flutter_test/flutter_test.dart';
import 'package:go_game/src/domain/models.dart';
import 'package:go_game/src/domain/puzzles/puzzle.dart';
import 'package:go_game/src/domain/puzzles/puzzle_session.dart';

void main() {
  group('Puzzle parsing', () {
    test('parses coordinates and players', () {
      final json = {
        'id': 'p',
        'title': 'T',
        'description': 'd',
        'boardSize': 9,
        'theme': 'capture',
        'difficulty': 1,
        'initial': {
          'black': ['ab'],
          'white': ['cd']
        },
        'toMove': 'black',
        'solution': [
          {
            'move': {'player': 'black', 'point': 'ef'},
            'outcome': 'correct',
          }
        ]
      };
      final p = Puzzle.fromJson(json);
      expect(p.initialBlack, [const Point(1, 0)]);
      expect(p.initialWhite, [const Point(3, 2)]);
      expect(p.toMove, StoneColor.black);
      expect(p.solution, hasLength(1));
      expect(p.solution.first.outcome, NodeOutcome.correct);
    });
  });

  group('PuzzleSession', () {
    Puzzle singleStoneAtari() => Puzzle.fromJson({
          'id': 'atari',
          'title': 'Atari',
          'description': 'capture the stone',
          'boardSize': 9,
          'theme': 'capture',
          'difficulty': 1,
          'initial': {
            'black': ['dc', 'cd', 'ed'],
            'white': ['dd']
          },
          'toMove': 'black',
          'solution': [
            {
              'move': {'player': 'black', 'point': 'de'},
              'outcome': 'correct',
            },
            {
              'move': {'player': 'black', 'point': 'aa'},
              'outcome': 'wrong',
            }
          ]
        });

    test('starts in progress, with black to move and white stone present', () {
      final s = PuzzleSession.start(singleStoneAtari());
      expect(s.status, PuzzleStatus.inProgress);
      expect(s.toMove, StoneColor.black);
      expect(s.state.board.cellAt(const Point(3, 3)), CellState.white);
    });

    test('correct move solves the puzzle and removes the captured stone', () {
      final s = PuzzleSession.start(singleStoneAtari());
      final ok = s.play(const Point(4, 3));
      expect(ok, isTrue);
      expect(s.status, PuzzleStatus.solved);
      expect(s.state.board.cellAt(const Point(3, 3)), CellState.empty);
    });

    test('off-tree moves count as mistakes but leave session in progress', () {
      final s = PuzzleSession.start(singleStoneAtari());
      final ok = s.play(const Point(8, 8));
      expect(ok, isFalse);
      expect(s.status, PuzzleStatus.inProgress);
      expect(s.mistakes, 1);
    });

    test('explicit wrong branch fails the session', () {
      final s = PuzzleSession.start(singleStoneAtari());
      final ok = s.play(const Point(0, 0));
      expect(ok, isTrue);
      expect(s.status, PuzzleStatus.failed);
    });

    test('hint reveals the next expected point', () {
      final s = PuzzleSession.start(singleStoneAtari());
      final hint = s.hint();
      expect(hint, const Point(4, 3));
      expect(s.hintsUsed, 1);
    });

    test('retry restores the original position', () {
      final s = PuzzleSession.start(singleStoneAtari());
      s.play(const Point(4, 3));
      expect(s.status, PuzzleStatus.solved);
      s.retry();
      expect(s.status, PuzzleStatus.inProgress);
      expect(s.state.board.cellAt(const Point(3, 3)), CellState.white);
    });
  });
}
