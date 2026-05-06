import 'dart:math' as math;
import 'dart:typed_data';
import 'models.dart';

/// Immutable square Go board, holding cell ordinals (0=empty, 1=black, 2=white).
class Board {
  final int size;
  final Uint8List _cells;

  Board._(this.size, this._cells);

  factory Board.empty(int size) => Board._(size, Uint8List(size * size));

  int index(int row, int col) => row * size + col;
  int indexOf(Point p) => index(p.row, p.col);

  bool inBounds(Point p) =>
      p.row >= 0 && p.row < size && p.col >= 0 && p.col < size;

  CellState cellAt(Point p) => CellState.values[_cells[indexOf(p)]];
  CellState cellAtRC(int row, int col) =>
      CellState.values[_cells[index(row, col)]];

  Board setCell(Point p, CellState state) {
    final copy = Uint8List.fromList(_cells);
    copy[indexOf(p)] = state.index;
    return Board._(size, copy);
  }

  Board setMany(Iterable<MapEntry<Point, CellState>> updates) {
    final list = updates.toList(growable: false);
    if (list.isEmpty) return this;
    final copy = Uint8List.fromList(_cells);
    for (final u in list) {
      copy[indexOf(u.key)] = u.value.index;
    }
    return Board._(size, copy);
  }

  List<Point> neighbors(Point p) {
    final out = <Point>[];
    if (p.row > 0) out.add(Point(p.row - 1, p.col));
    if (p.row < size - 1) out.add(Point(p.row + 1, p.col));
    if (p.col > 0) out.add(Point(p.row, p.col - 1));
    if (p.col < size - 1) out.add(Point(p.row, p.col + 1));
    return out;
  }

  /// Zobrist-style hash of the current position.
  ///
  /// Uses a 53-bit hash so that XOR composition stays exact on JS targets where
  /// integers are stored as doubles. Collision probability for any realistic Go
  /// game (a few thousand positions) is astronomically small.
  int zobristHash() {
    final table = _ZobristTable.forSize(size);
    var h = 0;
    for (var i = 0; i < _cells.length; i++) {
      final v = _cells[i];
      if (v != 0) {
        h ^= table[i * 2 + (v - 1)];
      }
    }
    return h;
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    if (other is! Board || other.size != size) return false;
    for (var i = 0; i < _cells.length; i++) {
      if (other._cells[i] != _cells[i]) return false;
    }
    return true;
  }

  @override
  int get hashCode {
    var h = size;
    for (var i = 0; i < _cells.length; i++) {
      h = (h * 31 + _cells[i]) & 0x3FFFFFFF;
    }
    return h;
  }
}

/// Internal: deterministic 53-bit hash table per board size, two entries per cell
/// (one per non-empty color).
class _ZobristTable {
  static const _mask53 = 0x1FFFFFFFFFFFFF;
  static final Map<int, List<int>> _cache = {};

  static List<int> forSize(int size) {
    return _cache.putIfAbsent(size, () {
      final rng = math.Random(0xCAFEBABE ^ size);
      final out = List<int>.filled(size * size * 2, 0, growable: false);
      for (var i = 0; i < out.length; i++) {
        // Combine two 32-bit draws into a 53-bit-safe integer.
        final hi = rng.nextInt(0x200000); // 21 bits
        final lo = rng.nextInt(0x100000000); // 32 bits
        out[i] = ((hi * 0x100000000) | lo) & _mask53;
      }
      return out;
    });
  }
}
