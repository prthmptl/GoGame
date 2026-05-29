import '../models.dart';

enum DrillCategory { capture, life, escape, attack }

extension DrillCategoryLabel on DrillCategory {
  String get label => switch (this) {
        DrillCategory.capture => 'Capture',
        DrillCategory.life => 'Life',
        DrillCategory.escape => 'Escape',
        DrillCategory.attack => 'Attack',
      };
}

class Drill {
  final String id;
  final String title;
  final String description;
  final DrillCategory category;
  final int boardSize;
  final List<Point> initialBlack;
  final List<Point> initialWhite;
  final StoneColor playerColor;
  final AiDifficulty aiLevel;
  final int requiredWins;

  const Drill({
    required this.id,
    required this.title,
    required this.description,
    required this.category,
    required this.boardSize,
    required this.initialBlack,
    required this.initialWhite,
    required this.playerColor,
    required this.aiLevel,
    required this.requiredWins,
  });

  static Drill fromJson(Map<String, dynamic> json) {
    final initial = json['initial'] as Map<String, dynamic>? ?? const {};
    return Drill(
      id: json['id'] as String,
      title: json['title'] as String? ?? 'Drill',
      description: json['description'] as String? ?? '',
      category: _parseCategory(json['category'] as String?),
      boardSize: json['boardSize'] as int? ?? 9,
      initialBlack: _parsePoints(initial['black']),
      initialWhite: _parsePoints(initial['white']),
      playerColor: json['playerColor'] == 'white'
          ? StoneColor.white
          : StoneColor.black,
      aiLevel: _parseAi(json['aiLevel'] as String?),
      requiredWins: json['requiredWins'] as int? ?? 3,
    );
  }

  static DrillCategory _parseCategory(String? s) {
    for (final c in DrillCategory.values) {
      if (c.name == s) return c;
    }
    return DrillCategory.capture;
  }

  static AiDifficulty _parseAi(String? s) {
    for (final d in AiDifficulty.values) {
      if (d.name == s) return d;
    }
    return AiDifficulty.beginner;
  }

  static List<Point> _parsePoints(Object? raw) {
    if (raw is! List) return const [];
    return raw.map((e) => _parsePoint(e as String)).toList(growable: false);
  }
}

Point _parsePoint(String s) {
  if (s.length != 2) {
    throw FormatException('Drill point must be 2 letters, got "$s"');
  }
  final col = s.codeUnitAt(0) - 0x61;
  final row = s.codeUnitAt(1) - 0x61;
  return Point(row, col);
}
