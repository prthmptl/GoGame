import '../models.dart';

enum BotStyle { aggressive, territorial, balanced, calm, tricky }

extension BotStyleLabel on BotStyle {
  String get label => switch (this) {
        BotStyle.aggressive => 'Aggressive',
        BotStyle.territorial => 'Territorial',
        BotStyle.balanced => 'Balanced',
        BotStyle.calm => 'Calm',
        BotStyle.tricky => 'Tricky',
      };
}

class BotProfile {
  final String id;
  final String name;
  final String countryEmoji;
  final int rating;
  final AiDifficulty engine;
  final BotStyle style;
  final String bio;

  /// Two-letter initials used as the avatar when no image asset is supplied.
  final String initials;

  const BotProfile({
    required this.id,
    required this.name,
    required this.countryEmoji,
    required this.rating,
    required this.engine,
    required this.style,
    required this.bio,
    required this.initials,
  });

  String get rankLabel {
    if (rating < 600) return '${(2000 - rating) ~/ 100}k';
    if (rating < 1000) return '${(1800 - rating) ~/ 100}k';
    if (rating < 1500) return '${(1700 - rating) ~/ 100}k';
    if (rating < 1900) return '${(1900 - rating) ~/ 100 + 1}k';
    if (rating < 2100) return '1d';
    if (rating < 2300) return '${(rating - 1900) ~/ 100}d';
    return '${(rating - 1900) ~/ 100}d';
  }
}
