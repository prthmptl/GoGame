package com.weiqi.sgf

import com.weiqi.engine.GameState
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.ScoreResult
import com.weiqi.engine.StoneColor
import java.time.LocalDate

object Sgf {

    /** Encode a board column index (0-based) to SGF letter (a, b, ...). SGF supports up to 25. */
    private fun coord(p: Point): String {
        val col = ('a' + p.col)
        val row = ('a' + p.row)
        return "$col$row"
    }

    fun export(
        state: GameState,
        score: ScoreResult? = null,
        blackName: String = "Black",
        whiteName: String = "White",
        date: LocalDate = LocalDate.now()
    ): String {
        val sb = StringBuilder()
        sb.append("(;GM[1]FF[4]CA[UTF-8]AP[Weiqi]")
        sb.append("SZ[").append(state.config.boardSize).append(']')
        sb.append("KM[").append(state.config.komi).append(']')
        sb.append("HA[").append(state.config.handicap).append(']')
        sb.append("RU[Chinese]")
        sb.append("PB[").append(blackName).append(']')
        sb.append("PW[").append(whiteName).append(']')
        sb.append("DT[").append(date).append(']')
        if (score != null) sb.append("RE[").append(score.resultString).append(']')

        for (m in state.history) {
            val tag = if (m.player == StoneColor.BLACK) "B" else "W"
            when (m.type) {
                MoveType.PLACE_STONE -> sb.append(';').append(tag).append('[').append(coord(m.point!!)).append(']')
                MoveType.PASS -> sb.append(';').append(tag).append("[]")
                MoveType.RESIGN -> { /* recorded via RE */ }
            }
        }
        sb.append(')')
        return sb.toString()
    }
}
