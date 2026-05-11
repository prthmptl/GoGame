import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/scoring.dart';

class Sgf {
  /// SGF RU[] property tags. See https://www.red-bean.com/sgf/properties.html#RU.
  static String _sgfRuleset(Ruleset r) {
    switch (r) {
      case Ruleset.chinese:
        return 'Chinese';
      case Ruleset.japanese:
        return 'Japanese';
      case Ruleset.korean:
        return 'Korean';
      case Ruleset.aga:
        return 'AGA';
      case Ruleset.ing:
        return 'Ing';
      case Ruleset.newZealand:
        return 'NZ';
      case Ruleset.trompTaylor:
        return 'TrompTaylor';
    }
  }

  static String _sgfComment(GameConfig cfg) {
    final defaults = RulesetDefaults.of(cfg.ruleset);
    final scoring = defaults.scoringMethod == ScoringMethod.area
        ? 'area scoring'
        : 'territory + prisoners scoring';
    final ko = switch (defaults.superkoMode) {
      SuperkoMode.none => 'basic ko',
      SuperkoMode.positional => 'positional superko',
      SuperkoMode.situational => 'situational superko',
    };
    final suicide = defaults.allowSuicide ? ', suicide legal' : '';
    // SGF text values must escape ']' and '\\'. The strings below are static
    // and never include those characters, so no escaping is required.
    return '${cfg.ruleset.label} rules — $scoring, komi ${cfg.komi}, $ko$suicide.';
  }

  static Ruleset rulesetFromSgf(String s) {
    switch (s.trim().toLowerCase()) {
      case 'japanese':
        return Ruleset.japanese;
      case 'korean':
        return Ruleset.korean;
      case 'aga':
        return Ruleset.aga;
      case 'ing':
      case 'goe':
        return Ruleset.ing;
      case 'nz':
      case 'new zealand':
        return Ruleset.newZealand;
      case 'tromptaylor':
      case 'tromp-taylor':
      case 'tromp taylor':
        return Ruleset.trompTaylor;
      case 'chinese':
      default:
        return Ruleset.chinese;
    }
  }

  /// Encode a Point to SGF coordinate letters (col, row).
  static String _coord(Point p) {
    final col = String.fromCharCode('a'.codeUnitAt(0) + p.col);
    final row = String.fromCharCode('a'.codeUnitAt(0) + p.row);
    return '$col$row';
  }

  static String export(
    GameState state, {
    ScoreResult? score,
    String blackName = 'Black',
    String whiteName = 'White',
    DateTime? date,
  }) {
    final d = date ?? DateTime.now();
    final dStr =
        '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
    final sb = StringBuffer();
    sb.write('(;GM[1]FF[4]CA[UTF-8]AP[Go]');
    sb
      ..write('SZ[')
      ..write(state.config.boardSize)
      ..write(']');
    sb
      ..write('KM[')
      ..write(state.config.komi)
      ..write(']');
    sb
      ..write('HA[')
      ..write(state.config.handicap)
      ..write(']');
    sb
      ..write('RU[')
      ..write(_sgfRuleset(state.config.ruleset))
      ..write(']');
    sb
      ..write('PB[')
      ..write(blackName)
      ..write(']');
    sb
      ..write('PW[')
      ..write(whiteName)
      ..write(']');
    sb
      ..write('DT[')
      ..write(dStr)
      ..write(']');
    if (score != null) {
      sb
        ..write('RE[')
        ..write(score.resultString)
        ..write(']');
    }
    // Human-readable game comment summarizing the ruleset. Most SGF viewers
    // surface GC[] alongside the player names, so the ruleset is visible
    // at-a-glance without inspecting the raw properties.
    sb
      ..write('GC[')
      ..write(_sgfComment(state.config))
      ..write(']');

    for (final m in state.history) {
      final tag = m.player == StoneColor.black ? 'B' : 'W';
      switch (m.type) {
        case MoveType.placeStone:
          sb
            ..write(';')
            ..write(tag)
            ..write('[')
            ..write(_coord(m.point!))
            ..write(']');
          break;
        case MoveType.pass:
          sb
            ..write(';')
            ..write(tag)
            ..write('[]');
          break;
        case MoveType.resign:
          // Recorded via RE[]; no move token.
          break;
      }
    }
    sb.write(')');
    return sb.toString();
  }
}
