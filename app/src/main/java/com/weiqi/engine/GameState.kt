package com.weiqi.engine

data class GameState(
    val board: Board,
    val config: GameConfig,
    val currentPlayer: StoneColor,
    val moveNumber: Int,
    val capturesByBlack: Int,
    val capturesByWhite: Int,
    val koPoint: Point?,
    val previousHashes: Set<Long>,
    val status: GameStatus,
    val consecutivePasses: Int,
    val lastMove: Move?,
    val history: List<Move>
) {
    companion object {
        fun newGame(config: GameConfig): GameState {
            val board = Board.empty(config.boardSize)
            val starting = if (config.handicap > 0) StoneColor.WHITE else StoneColor.BLACK
            val withHandicap = applyHandicap(board, config.handicap)
            return GameState(
                board = withHandicap,
                config = config,
                currentPlayer = starting,
                moveNumber = 0,
                capturesByBlack = 0,
                capturesByWhite = 0,
                koPoint = null,
                previousHashes = setOf(withHandicap.zobristHash()),
                status = GameStatus.ACTIVE,
                consecutivePasses = 0,
                lastMove = null,
                history = emptyList()
            )
        }

        private fun applyHandicap(board: Board, handicap: Int): Board {
            if (handicap <= 0) return board
            val pts = handicapPoints(board.size, handicap)
            return board.setMany(pts.map { it to CellState.BLACK })
        }

        private fun handicapPoints(size: Int, n: Int): List<Point> {
            if (size != 9 && size != 13 && size != 19) return emptyList()
            val edge = if (size == 9) 2 else 3
            val far = size - 1 - edge
            val mid = size / 2
            val star = listOf(
                Point(edge, edge), Point(far, far), Point(edge, far), Point(far, edge),
                Point(mid, mid), Point(mid, edge), Point(mid, far),
                Point(edge, mid), Point(far, mid)
            )
            return star.take(n.coerceIn(0, 9))
        }
    }
}
