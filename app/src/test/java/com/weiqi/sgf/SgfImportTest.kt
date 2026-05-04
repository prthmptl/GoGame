package com.weiqi.sgf

import com.weiqi.engine.CellState
import com.weiqi.engine.Point
import org.junit.Assert.assertEquals
import org.junit.Test

class SgfImportTest {
    @Test fun importsHeaderAndMoves() {
        val sgf = "(;GM[1]FF[4]SZ[9]KM[7.5]HA[0];B[dc];W[ed];B[];W[ef])"
        val s = SgfImport.import(sgf)
        assertEquals(9, s.config.boardSize)
        assertEquals(7.5, s.config.komi, 1e-9)
        assertEquals(CellState.BLACK, s.board.get(Point(2, 3)))    // dc -> col d (3), row c (2)
        assertEquals(CellState.WHITE, s.board.get(Point(3, 4)))
        assertEquals(CellState.WHITE, s.board.get(Point(5, 4)))
    }
}
