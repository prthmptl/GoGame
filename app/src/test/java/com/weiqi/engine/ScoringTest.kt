package com.weiqi.engine

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ScoringTest {

    @Test fun emptyBoardKomiOnly() {
        val s = GameState.newGame(GameConfig(boardSize = 9, komi = 7.5))
        val res = Scoring.score(s)
        assertEquals(0, res.blackArea)
        assertEquals(7.5, res.whiteTotal, 1e-9)
        assertTrue(res.resultString.startsWith("W+"))
    }

    @Test fun blackWallTerritory() {
        // Build a black wall enclosing a 3x3 region in the corner on a 9x9.
        var s = GameState.newGame(GameConfig(boardSize = 9, komi = 0.0))
        // Black plays the wall row=3 across cols 0..3 and col=3 down rows 0..3, white plays useless stones.
        val blackPts = listOf(
            Point(3, 0), Point(3, 1), Point(3, 2), Point(3, 3),
            Point(0, 3), Point(1, 3), Point(2, 3)
        )
        val whitePts = listOf(
            Point(8, 8), Point(8, 7), Point(7, 8), Point(7, 7),
            Point(8, 6), Point(6, 8), Point(6, 7)
        )
        // Alternate moves manually.
        var bi = 0; var wi = 0
        while (bi < blackPts.size || wi < whitePts.size) {
            if (bi < blackPts.size) {
                val r = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, blackPts[bi]))
                s = (r as MoveResult.Accepted).newState; bi++
            }
            if (wi < whitePts.size) {
                val r = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, whitePts[wi]))
                s = (r as MoveResult.Accepted).newState; wi++
            }
        }
        val score = Scoring.score(s)
        // 3x3 = 9 empty intersections enclosed by black + 7 black stones + 7 white stones, neutral zero ideally.
        assertEquals(9, score.blackTerritory)
        assertEquals(7, score.blackStones)
        assertEquals(7, score.whiteStones)
        // Black area = 7 + 9 = 16. White total = 7 + 0 + 0 = 7. Black wins by 9.
        assertEquals(16, score.blackArea)
        assertEquals(7.0, score.whiteTotal, 1e-9)
    }

    @Test fun deadStonesAreRemoved() {
        var s = GameState.newGame(GameConfig(boardSize = 9, komi = 0.0))
        s = (Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(4, 4))) as MoveResult.Accepted).newState
        // Mark the lone black stone as dead — it should be removed before counting.
        val score = Scoring.score(s, deadStones = setOf(Point(4, 4)))
        assertEquals(0, score.blackStones)
    }
}
