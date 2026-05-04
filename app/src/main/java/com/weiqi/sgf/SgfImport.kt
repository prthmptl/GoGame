package com.weiqi.sgf

import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.Ruleset

object SgfImport {

    data class ParsedHeader(
        val size: Int = 19,
        val komi: Double = 7.5,
        val handicap: Int = 0,
    )

    /** Parse the linear main line of an SGF and replay it. Variations are ignored. */
    fun import(sgf: String): GameState {
        val header = parseHeader(sgf)
        var state = GameState.newGame(
            GameConfig(
                boardSize = header.size,
                ruleset = Ruleset.CHINESE,
                komi = header.komi,
                handicap = header.handicap
            )
        )
        for (intent in extractMoves(sgf, header.size)) {
            val r = Rules.apply(state, intent)
            if (r is MoveResult.Accepted) state = r.newState else break
        }
        return state
    }

    private fun parseHeader(sgf: String): ParsedHeader {
        val sz = Regex("""SZ\[(\d+)]""").find(sgf)?.groupValues?.get(1)?.toIntOrNull() ?: 19
        val km = Regex("""KM\[([0-9.+\-]+)]""").find(sgf)?.groupValues?.get(1)?.toDoubleOrNull() ?: 7.5
        val ha = Regex("""HA\[(\d+)]""").find(sgf)?.groupValues?.get(1)?.toIntOrNull() ?: 0
        return ParsedHeader(sz, km, ha)
    }

    private fun extractMoves(sgf: String, size: Int): List<MoveIntent> {
        val out = ArrayList<MoveIntent>()
        // Skip everything up to the first move marker. Property tokens we care about: ;B[..] and ;W[..].
        val moveRegex = Regex("""[;\(]([BW])\[([a-zA-Z]{0,2})]""")
        for (m in moveRegex.findAll(sgf)) {
            val coord = m.groupValues[2]
            if (coord.isEmpty() || coord == "tt") {
                out.add(MoveIntent(MoveType.PASS))
            } else if (coord.length >= 2) {
                val col = coord[0].lowercaseChar() - 'a'
                val row = coord[1].lowercaseChar() - 'a'
                if (col in 0 until size && row in 0 until size) {
                    out.add(MoveIntent(MoveType.PLACE_STONE, Point(row, col)))
                }
            }
        }
        return out
    }
}
