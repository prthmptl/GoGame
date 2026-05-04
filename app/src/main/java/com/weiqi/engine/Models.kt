package com.weiqi.engine

enum class StoneColor { BLACK, WHITE;
    fun other(): StoneColor = if (this == BLACK) WHITE else BLACK
}

enum class CellState { EMPTY, BLACK, WHITE;
    companion object {
        fun of(color: StoneColor) = if (color == StoneColor.BLACK) BLACK else WHITE
    }
}

data class Point(val row: Int, val col: Int)

enum class MoveType { PLACE_STONE, PASS, RESIGN }

data class Move(
    val moveNumber: Int,
    val player: StoneColor,
    val type: MoveType,
    val point: Point?,
    val captured: List<Point>
)

enum class Ruleset { CHINESE }

data class GameConfig(
    val boardSize: Int,
    val ruleset: Ruleset = Ruleset.CHINESE,
    val komi: Double = 7.5,
    val handicap: Int = 0,
    val allowSuicide: Boolean = false,
    val useSuperko: Boolean = true
)

enum class GameStatus { ACTIVE, SCORING, COMPLETED, RESIGNED }

sealed class MoveResult {
    data class Accepted(val newState: GameState, val move: Move) : MoveResult()
    data class Rejected(val reason: Reason) : MoveResult()

    enum class Reason {
        GAME_NOT_ACTIVE, OUT_OF_BOUNDS, OCCUPIED, SUICIDE, KO_VIOLATION, SUPERKO_VIOLATION
    }
}

data class MoveIntent(val type: MoveType, val point: Point? = null)
