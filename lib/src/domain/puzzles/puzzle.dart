import '../models.dart';

enum PuzzleTheme {
  capture,
  lifeAndDeath,
  tesuji,
  ko,
  endgame,
  opening,
}

extension PuzzleThemeLabel on PuzzleTheme {
  String get label => switch (this) {
        PuzzleTheme.capture => 'Capture',
        PuzzleTheme.lifeAndDeath => 'Life & Death',
        PuzzleTheme.tesuji => 'Tesuji',
        PuzzleTheme.ko => 'Ko',
        PuzzleTheme.endgame => 'Endgame',
        PuzzleTheme.opening => 'Opening',
      };

  static PuzzleTheme parse(String s) {
    for (final t in PuzzleTheme.values) {
      if (t.name == s) return t;
    }
    return PuzzleTheme.capture;
  }
}

/// A single move inside a puzzle's solution tree.
class PuzzleMove {
  final StoneColor player;
  final Point point;

  const PuzzleMove({required this.player, required this.point});

  static PuzzleMove fromJson(Map<String, dynamic> json) => PuzzleMove(
        player: json['player'] == 'white' ? StoneColor.white : StoneColor.black,
        point: _parsePoint(json['point'] as String),
      );
}

/// Outcome of reaching a particular [PuzzleNode]:
/// - [continues]: legal partial answer; opponent will reply.
/// - [correct]: this branch completes the puzzle successfully.
/// - [wrong]: this branch ends the puzzle as a failure.
enum NodeOutcome { continues, correct, wrong }

class PuzzleNode {
  final PuzzleMove move;
  final NodeOutcome outcome;
  final String? comment;
  final List<PuzzleNode> children;

  const PuzzleNode({
    required this.move,
    required this.outcome,
    this.comment,
    this.children = const [],
  });

  static PuzzleNode fromJson(Map<String, dynamic> json) => PuzzleNode(
        move: PuzzleMove.fromJson(json['move'] as Map<String, dynamic>),
        outcome: _parseOutcome(json['outcome'] as String?),
        comment: json['comment'] as String?,
        children: (json['children'] as List<dynamic>? ?? const [])
            .map((e) => PuzzleNode.fromJson(e as Map<String, dynamic>))
            .toList(growable: false),
      );

  static NodeOutcome _parseOutcome(String? s) => switch (s) {
        'correct' => NodeOutcome.correct,
        'wrong' => NodeOutcome.wrong,
        _ => NodeOutcome.continues,
      };
}

class Puzzle {
  final String id;
  final String title;
  final String description;
  final int boardSize;
  final PuzzleTheme theme;
  final int difficulty;
  final List<Point> initialBlack;
  final List<Point> initialWhite;
  final StoneColor toMove;
  final List<PuzzleNode> solution;

  const Puzzle({
    required this.id,
    required this.title,
    required this.description,
    required this.boardSize,
    required this.theme,
    required this.difficulty,
    required this.initialBlack,
    required this.initialWhite,
    required this.toMove,
    required this.solution,
  });

  static Puzzle fromJson(Map<String, dynamic> json) {
    final initial = json['initial'] as Map<String, dynamic>? ?? const {};
    final solution = (json['solution'] as List<dynamic>? ?? const [])
        .map((e) => PuzzleNode.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
    return Puzzle(
      id: json['id'] as String,
      title: json['title'] as String? ?? 'Puzzle',
      description: json['description'] as String? ?? '',
      boardSize: json['boardSize'] as int? ?? 9,
      theme: PuzzleThemeLabel.parse(json['theme'] as String? ?? 'capture'),
      difficulty: json['difficulty'] as int? ?? 1,
      initialBlack: _parsePoints(initial['black']),
      initialWhite: _parsePoints(initial['white']),
      toMove: json['toMove'] == 'white' ? StoneColor.white : StoneColor.black,
      solution: solution,
    );
  }

  static List<Point> _parsePoints(Object? raw) {
    if (raw is! List) return const [];
    return raw
        .map((e) => _parsePoint(e as String))
        .toList(growable: false);
  }
}

/// SGF-style 2-letter coordinates: column letter, then row letter, both `a`=0.
/// `aa` is the top-left corner.
Point _parsePoint(String s) {
  if (s.length != 2) {
    throw FormatException('Puzzle point must be 2 letters, got "$s"');
  }
  final col = s.codeUnitAt(0) - 0x61;
  final row = s.codeUnitAt(1) - 0x61;
  return Point(row, col);
}
