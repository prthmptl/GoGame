package com.weiqi.engine

data class ScoreResult(
    val blackStones: Int,
    val whiteStones: Int,
    val blackTerritory: Int,
    val whiteTerritory: Int,
    val neutral: Int,
    val komi: Double
) {
    val blackArea: Int get() = blackStones + blackTerritory
    val whiteTotal: Double get() = (whiteStones + whiteTerritory) + komi
    /** Positive = black wins by N. Negative = white wins by |N|. */
    val margin: Double get() = blackArea - whiteTotal
    val resultString: String get() = when {
        margin > 0 -> "B+%.1f".format(margin)
        margin < 0 -> "W+%.1f".format(-margin)
        else -> "Draw"
    }
}

object Scoring {

    /**
     * Chinese area scoring.
     * @param deadStones points whose stones are agreed dead — they are removed before counting.
     */
    fun score(state: GameState, deadStones: Set<Point> = emptySet()): ScoreResult {
        val board = removeDead(state.board, deadStones)
        val size = board.size

        var blackStones = 0
        var whiteStones = 0
        for (r in 0 until size) for (c in 0 until size) {
            when (board.get(r, c)) {
                CellState.BLACK -> blackStones++
                CellState.WHITE -> whiteStones++
                else -> {}
            }
        }

        val visited = BooleanArray(size * size)
        var blackTerritory = 0
        var whiteTerritory = 0
        var neutral = 0
        for (r in 0 until size) for (c in 0 until size) {
            val idx = board.index(r, c)
            if (visited[idx]) continue
            if (board.get(r, c) != CellState.EMPTY) {
                visited[idx] = true; continue
            }
            // Flood-fill the empty region.
            val region = ArrayList<Point>()
            val borders = HashSet<CellState>()
            val stack = ArrayDeque<Point>()
            stack.addLast(Point(r, c))
            while (stack.isNotEmpty()) {
                val p = stack.removeLast()
                val pi = board.index(p)
                if (visited[pi]) continue
                visited[pi] = true
                region.add(p)
                for (n in board.neighbors(p)) {
                    val st = board.get(n)
                    if (st == CellState.EMPTY) {
                        if (!visited[board.index(n)]) stack.addLast(n)
                    } else borders.add(st)
                }
            }
            when {
                borders == setOf(CellState.BLACK) -> blackTerritory += region.size
                borders == setOf(CellState.WHITE) -> whiteTerritory += region.size
                else -> neutral += region.size
            }
        }

        return ScoreResult(
            blackStones = blackStones,
            whiteStones = whiteStones,
            blackTerritory = blackTerritory,
            whiteTerritory = whiteTerritory,
            neutral = neutral,
            komi = state.config.komi
        )
    }

    private fun removeDead(board: Board, dead: Set<Point>): Board {
        if (dead.isEmpty()) return board
        return board.setMany(dead.map { it to CellState.EMPTY })
    }
}
