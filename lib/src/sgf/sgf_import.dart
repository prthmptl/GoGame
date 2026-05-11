import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/rules.dart';
import 'sgf.dart';

class SgfParsedHeader {
  final int size;
  final double komi;
  final int handicap;
  final Ruleset ruleset;
  const SgfParsedHeader({
    this.size = 19,
    this.komi = 7.5,
    this.handicap = 0,
    this.ruleset = Ruleset.chinese,
  });
}

class SgfImport {
  /// Parse the linear main line of an SGF and replay it. Variations are ignored.
  static GameState import(String sgf) {
    final header = _parseHeader(sgf);
    final defaults = RulesetDefaults.of(header.ruleset);
    var state = GameState.newGame(GameConfig(
      boardSize: header.size,
      ruleset: header.ruleset,
      komi: header.komi,
      handicap: header.handicap,
      allowSuicide: defaults.allowSuicide,
      superkoMode: defaults.superkoMode,
    ));
    for (final intent in _extractMoves(sgf, header.size)) {
      final r = Rules.apply(state, intent);
      if (r.isAccepted) {
        state = r.newStateAs<GameState>();
      } else {
        break;
      }
    }
    return state;
  }

  static SgfParsedHeader _parseHeader(String sgf) {
    int size = 19;
    double komi = 7.5;
    int handicap = 0;
    Ruleset ruleset = Ruleset.chinese;
    final szMatch = RegExp(r'SZ\[(\d+)]').firstMatch(sgf);
    if (szMatch != null) size = int.tryParse(szMatch.group(1)!) ?? 19;
    final kmMatch = RegExp(r'KM\[([0-9.+\-]+)]').firstMatch(sgf);
    if (kmMatch != null) komi = double.tryParse(kmMatch.group(1)!) ?? 7.5;
    final haMatch = RegExp(r'HA\[(\d+)]').firstMatch(sgf);
    if (haMatch != null) handicap = int.tryParse(haMatch.group(1)!) ?? 0;
    final ruMatch = RegExp(r'RU\[([^\]]+)]').firstMatch(sgf);
    if (ruMatch != null) ruleset = Sgf.rulesetFromSgf(ruMatch.group(1)!);
    return SgfParsedHeader(
        size: size, komi: komi, handicap: handicap, ruleset: ruleset);
  }

  static List<MoveIntent> _extractMoves(String sgf, int size) {
    final out = <MoveIntent>[];
    final moveRegex = RegExp(r'[;(]([BW])\[([a-zA-Z]{0,2})]');
    final aBase = 'a'.codeUnitAt(0);
    for (final m in moveRegex.allMatches(sgf)) {
      final coord = m.group(2)!;
      if (coord.isEmpty || coord == 'tt') {
        out.add(const MoveIntent.pass());
      } else if (coord.length >= 2) {
        final col = coord.codeUnitAt(0).toLowerCase() - aBase;
        final row = coord.codeUnitAt(1).toLowerCase() - aBase;
        if (col >= 0 && col < size && row >= 0 && row < size) {
          out.add(MoveIntent.place(Point(row, col)));
        }
      }
    }
    return out;
  }
}

extension on int {
  int toLowerCase() {
    // ASCII A-Z to a-z; otherwise unchanged.
    if (this >= 0x41 && this <= 0x5A) return this + 0x20;
    return this;
  }
}
