import '../models.dart';
import 'bot_profile.dart';

/// Hand-curated roster of named opponents. The roster is intentionally diverse
/// so the picker feels alive even on first launch. Engine + style determine
/// real play; the rest is presentation.
class BotCatalog {
  static const all = <BotProfile>[
    BotProfile(
      id: 'kiri',
      name: 'Kiri',
      countryEmoji: '🇯🇵',
      rating: 700,
      engine: AiDifficulty.beginner,
      style: BotStyle.calm,
      bio: 'Just learning the ropes. Plays the corners and tries to live.',
      initials: 'KI',
    ),
    BotProfile(
      id: 'mei',
      name: 'Mei',
      countryEmoji: '🇨🇳',
      rating: 850,
      engine: AiDifficulty.beginner,
      style: BotStyle.aggressive,
      bio: 'Will attack a weak stone before thinking it through.',
      initials: 'ME',
    ),
    BotProfile(
      id: 'ari',
      name: 'Ari',
      countryEmoji: '🇰🇷',
      rating: 950,
      engine: AiDifficulty.beginner,
      style: BotStyle.territorial,
      bio: 'Loves third-line moves. Tries to fence off the corner early.',
      initials: 'AR',
    ),
    BotProfile(
      id: 'nori',
      name: 'Nori',
      countryEmoji: '🇯🇵',
      rating: 1100,
      engine: AiDifficulty.beginner,
      style: BotStyle.tricky,
      bio: 'Will throw in a weird shoulder hit to see if you can answer it.',
      initials: 'NO',
    ),
    BotProfile(
      id: 'sora',
      name: 'Sora',
      countryEmoji: '🇯🇵',
      rating: 1250,
      engine: AiDifficulty.intermediate,
      style: BotStyle.balanced,
      bio: 'Plays solid Go with a quiet smile. Punishes overplays.',
      initials: 'SO',
    ),
    BotProfile(
      id: 'leo',
      name: 'Leo',
      countryEmoji: '🇪🇸',
      rating: 1350,
      engine: AiDifficulty.intermediate,
      style: BotStyle.aggressive,
      bio: 'Picks fights early, especially if you split his stones.',
      initials: 'LE',
    ),
    BotProfile(
      id: 'rin',
      name: 'Rin',
      countryEmoji: '🇨🇳',
      rating: 1450,
      engine: AiDifficulty.intermediate,
      style: BotStyle.territorial,
      bio: 'Quiet endgame specialist. Will close every dame with feeling.',
      initials: 'RI',
    ),
    BotProfile(
      id: 'oskar',
      name: 'Oskar',
      countryEmoji: '🇩🇪',
      rating: 1550,
      engine: AiDifficulty.intermediate,
      style: BotStyle.calm,
      bio: 'Reads three moves ahead and never plays a panic move.',
      initials: 'OS',
    ),
    BotProfile(
      id: 'amara',
      name: 'Amara',
      countryEmoji: '🇳🇬',
      rating: 1650,
      engine: AiDifficulty.intermediate,
      style: BotStyle.tricky,
      bio: 'Specializes in tesuji and surprise sacrifices.',
      initials: 'AM',
    ),
    BotProfile(
      id: 'tomi',
      name: 'Tomi',
      countryEmoji: '🇫🇮',
      rating: 1750,
      engine: AiDifficulty.advanced,
      style: BotStyle.balanced,
      bio: 'Plays AlphaGo-flavored Go: shoulder hits and 3-3 invasions.',
      initials: 'TO',
    ),
    BotProfile(
      id: 'jun',
      name: 'Jun',
      countryEmoji: '🇰🇷',
      rating: 1850,
      engine: AiDifficulty.advanced,
      style: BotStyle.aggressive,
      bio: 'Famous for killing groups that look perfectly alive.',
      initials: 'JU',
    ),
    BotProfile(
      id: 'lina',
      name: 'Lina',
      countryEmoji: '🇧🇷',
      rating: 1950,
      engine: AiDifficulty.advanced,
      style: BotStyle.territorial,
      bio: 'Plays huge moyos and dares you to invade.',
      initials: 'LI',
    ),
    BotProfile(
      id: 'sage',
      name: 'Sage',
      countryEmoji: '🇨🇦',
      rating: 2050,
      engine: AiDifficulty.advanced,
      style: BotStyle.calm,
      bio: 'Will close out a 0.5-point endgame from move 40.',
      initials: 'SA',
    ),
    BotProfile(
      id: 'kazu',
      name: 'Kazu',
      countryEmoji: '🇯🇵',
      rating: 2200,
      engine: AiDifficulty.advanced,
      style: BotStyle.balanced,
      bio: 'A polished all-rounder. Few weak points, fewer mistakes.',
      initials: 'KA',
    ),
    BotProfile(
      id: 'naga',
      name: 'Naga',
      countryEmoji: '🇮🇳',
      rating: 2300,
      engine: AiDifficulty.advanced,
      style: BotStyle.tricky,
      bio: 'The boss. Bring an opening you actually know.',
      initials: 'NA',
    ),
  ];

  static BotProfile? byId(String id) {
    for (final b in all) {
      if (b.id == id) return b;
    }
    return null;
  }

  /// Default suggestion ordered by rating for a fresh player.
  static List<BotProfile> sortedByRating() =>
      [...all]..sort((a, b) => a.rating.compareTo(b.rating));
}
