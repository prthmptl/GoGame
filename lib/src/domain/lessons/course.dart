import '../models.dart';

enum SectionKind { text, diagram, exercise }

abstract class LessonSection {
  const LessonSection();
  SectionKind get kind;

  static LessonSection fromJson(Map<String, dynamic> json) {
    switch (json['kind']) {
      case 'diagram':
        return DiagramSection.fromJson(json);
      case 'exercise':
        return ExerciseSection.fromJson(json);
      default:
        return TextSection.fromJson(json);
    }
  }
}

class TextSection extends LessonSection {
  final String body;
  const TextSection(this.body);
  @override
  SectionKind get kind => SectionKind.text;

  static TextSection fromJson(Map<String, dynamic> json) =>
      TextSection(json['body'] as String? ?? '');
}

class DiagramSection extends LessonSection {
  final int boardSize;
  final List<Point> black;
  final List<Point> white;
  final List<Point> markers;
  final String? caption;

  const DiagramSection({
    required this.boardSize,
    required this.black,
    required this.white,
    required this.markers,
    this.caption,
  });

  @override
  SectionKind get kind => SectionKind.diagram;

  static DiagramSection fromJson(Map<String, dynamic> json) => DiagramSection(
        boardSize: json['boardSize'] as int? ?? 9,
        black: _parsePoints(json['black']),
        white: _parsePoints(json['white']),
        markers: _parsePoints(json['markers']),
        caption: json['caption'] as String?,
      );
}

class ExerciseSection extends LessonSection {
  final String puzzleId;
  final String? prompt;
  const ExerciseSection({required this.puzzleId, this.prompt});
  @override
  SectionKind get kind => SectionKind.exercise;

  static ExerciseSection fromJson(Map<String, dynamic> json) =>
      ExerciseSection(
        puzzleId: json['puzzleId'] as String,
        prompt: json['prompt'] as String?,
      );
}

class Lesson {
  final String id;
  final String title;
  final String summary;
  final List<LessonSection> sections;

  const Lesson({
    required this.id,
    required this.title,
    required this.summary,
    required this.sections,
  });

  static Lesson fromJson(Map<String, dynamic> json) => Lesson(
        id: json['id'] as String,
        title: json['title'] as String? ?? 'Lesson',
        summary: json['summary'] as String? ?? '',
        sections: (json['sections'] as List<dynamic>? ?? const [])
            .map((e) => LessonSection.fromJson(e as Map<String, dynamic>))
            .toList(growable: false),
      );
}

class Course {
  final String id;
  final String title;
  final String subtitle;
  final String description;
  final List<Lesson> lessons;

  const Course({
    required this.id,
    required this.title,
    required this.subtitle,
    required this.description,
    required this.lessons,
  });

  static Course fromJson(Map<String, dynamic> json) => Course(
        id: json['id'] as String,
        title: json['title'] as String? ?? 'Course',
        subtitle: json['subtitle'] as String? ?? '',
        description: json['description'] as String? ?? '',
        lessons: (json['lessons'] as List<dynamic>? ?? const [])
            .map((e) => Lesson.fromJson(e as Map<String, dynamic>))
            .toList(growable: false),
      );
}

List<Point> _parsePoints(Object? raw) {
  if (raw is! List) return const [];
  return raw.map((e) => _parsePoint(e as String)).toList(growable: false);
}

Point _parsePoint(String s) {
  if (s.length != 2) {
    throw FormatException('Lesson point must be 2 letters, got "$s"');
  }
  final col = s.codeUnitAt(0) - 0x61;
  final row = s.codeUnitAt(1) - 0x61;
  return Point(row, col);
}
