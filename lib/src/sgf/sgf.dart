import '../domain/game_state.dart';
import '../domain/models.dart';
import '../domain/scoring.dart';

class Sgf {
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
    sb.write('(;GM[1]FF[4]CA[UTF-8]AP[Weiqi]');
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
    sb.write('RU[Chinese]');
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
