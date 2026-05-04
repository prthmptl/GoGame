package com.weiqi.engine

class Board private constructor(
    val size: Int,
    private val cells: IntArray
) {
    fun index(row: Int, col: Int) = row * size + col
    fun index(p: Point) = index(p.row, p.col)
    fun inBounds(p: Point) = p.row in 0 until size && p.col in 0 until size
    fun get(p: Point): CellState = CellState.entries[cells[index(p)]]
    fun get(row: Int, col: Int): CellState = CellState.entries[cells[index(row, col)]]

    fun set(p: Point, state: CellState): Board {
        val copy = cells.copyOf()
        copy[index(p)] = state.ordinal
        return Board(size, copy)
    }

    fun setMany(updates: List<Pair<Point, CellState>>): Board {
        if (updates.isEmpty()) return this
        val copy = cells.copyOf()
        for ((p, s) in updates) copy[index(p)] = s.ordinal
        return Board(size, copy)
    }

    fun neighbors(p: Point): List<Point> {
        val out = ArrayList<Point>(4)
        if (p.row > 0) out.add(Point(p.row - 1, p.col))
        if (p.row < size - 1) out.add(Point(p.row + 1, p.col))
        if (p.col > 0) out.add(Point(p.row, p.col - 1))
        if (p.col < size - 1) out.add(Point(p.row, p.col + 1))
        return out
    }

    fun rawCells(): IntArray = cells.copyOf()

    /** Zobrist hash of the current position. */
    fun zobristHash(): Long {
        val table = ZobristTable.forSize(size)
        var h = 0L
        for (i in cells.indices) {
            val v = cells[i]
            if (v != 0) h = h xor table[i][v - 1]
        }
        return h
    }

    override fun equals(other: Any?): Boolean =
        other is Board && other.size == size && other.cells.contentEquals(cells)

    override fun hashCode(): Int = cells.contentHashCode() * 31 + size

    companion object {
        fun empty(size: Int): Board = Board(size, IntArray(size * size))
    }
}

private object ZobristTable {
    private val cache = HashMap<Int, Array<LongArray>>()
    fun forSize(size: Int): Array<LongArray> = cache.getOrPut(size) {
        val rng = java.util.Random(0xCAFEBABEL xor size.toLong())
        Array(size * size) { LongArray(2) { rng.nextLong() } }
    }
}
