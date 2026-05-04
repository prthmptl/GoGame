package com.weiqi.sgf

import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import java.time.LocalDate
import org.junit.Assert.assertTrue
import org.junit.Test

class SgfTest {

    @Test fun exportsBasicGame() {
        var s = GameState.newGame(GameConfig(boardSize = 9, komi = 7.5))
        s = (Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(2, 3))) as MoveResult.Accepted).newState
        s = (Rules.apply(s, MoveIntent(MoveType.PASS)) as MoveResult.Accepted).newState
        val sgf = Sgf.export(s, score = null, blackName = "B", whiteName = "W", date = LocalDate.of(2026, 5, 3))
        assertTrue(sgf.startsWith("(;GM[1]FF[4]"))
        assertTrue(sgf.contains("SZ[9]"))
        assertTrue(sgf.contains("KM[7.5]"))
        assertTrue(sgf.contains("RU[Chinese]"))
        assertTrue(sgf.contains(";B[dc]"))   // (row=2,col=3) -> col 'd', row 'c'
        assertTrue(sgf.contains(";W[]"))      // pass
        assertTrue(sgf.endsWith(")"))
    }
}
