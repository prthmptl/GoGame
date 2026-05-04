package com.weiqi.engine

internal data class Group(val stones: Set<Point>, val liberties: Set<Point>)

internal object Groups {
    fun findGroup(board: Board, start: Point): Group {
        val color = board.get(start)
        require(color != CellState.EMPTY)
        val stones = HashSet<Point>()
        val libs = HashSet<Point>()
        val stack = ArrayDeque<Point>()
        stack.addLast(start)
        while (stack.isNotEmpty()) {
            val p = stack.removeLast()
            if (!stones.add(p)) continue
            for (n in board.neighbors(p)) {
                val s = board.get(n)
                when (s) {
                    CellState.EMPTY -> libs.add(n)
                    color -> if (n !in stones) stack.addLast(n)
                    else -> {}
                }
            }
        }
        return Group(stones, libs)
    }
}
