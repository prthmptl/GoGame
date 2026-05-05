package com.weiqi.engine

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RulesTest {

    private fun newGame(size: Int = 9, komi: Double = 7.5) =
        GameState.newGame(GameConfig(boardSize = size, komi = komi))

    private fun play(state: GameState, r: Int, c: Int): GameState {
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, Point(r, c)))
        assertTrue("expected accepted at ($r,$c) but got $res", res is MoveResult.Accepted)
        return (res as MoveResult.Accepted).newState
    }

    private fun pass(state: GameState): GameState {
        val res = Rules.apply(state, MoveIntent(MoveType.PASS))
        return (res as MoveResult.Accepted).newState
    }

    @Test fun basicPlacement() {
        var s = newGame()
        s = play(s, 4, 4)
        assertEquals(CellState.BLACK, s.board.get(Point(4, 4)))
        assertEquals(StoneColor.WHITE, s.currentPlayer)
    }

    @Test fun rejectOccupied() {
        var s = newGame()
        s = play(s, 4, 4)
        val res = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(4, 4)))
        assertEquals(MoveResult.Rejected(MoveResult.Reason.OCCUPIED), res)
    }

    @Test fun rejectOutOfBounds() {
        val s = newGame(9)
        val res = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(9, 0)))
        assertEquals(MoveResult.Rejected(MoveResult.Reason.OUT_OF_BOUNDS), res)
    }

    @Test fun singleStoneCapture() {
        // White stone at (1,1) surrounded by black on all four sides.
        var s = newGame()
        s = play(s, 1, 1) // B
        // To make white go on (2,2) trapped, we need a different setup.
        // Easier: black plays around a white stone in the corner.
        s = newGame()
        s = play(s, 0, 1) // B
        s = play(s, 0, 0) // W (corner)
        s = play(s, 1, 0) // B  -> captures W at (0,0)
        assertEquals(CellState.EMPTY, s.board.get(Point(0, 0)))
        assertEquals(1, s.capturesByBlack)
    }

    @Test fun rejectSuicide() {
        // Surround (0,0) with white stones, then black plays (0,0): suicide.
        var s = newGame()
        s = play(s, 5, 5) // B (away)
        s = play(s, 0, 1) // W
        s = play(s, 5, 6) // B
        s = play(s, 1, 0) // W
        // Black to move at (0,0) — surrounded by white, no captures.
        val res = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(0, 0)))
        assertEquals(MoveResult.Rejected(MoveResult.Reason.SUICIDE), res)
    }

    @Test fun captureBeatsApparentSuicide() {
        // White group with 1 liberty; black plays into that liberty and captures, not suicide.
        var s = newGame()
        // Build white group at (0,0)-(0,1) with last liberty (0,2).
        s = play(s, 1, 0) // B
        s = play(s, 0, 0) // W
        s = play(s, 1, 1) // B
        s = play(s, 0, 1) // W
        s = play(s, 1, 2) // B
        s = play(s, 8, 8) // W away
        // Now black plays (0,2): captures white group.
        s = play(s, 0, 2) // B
        assertEquals(CellState.EMPTY, s.board.get(Point(0, 0)))
        assertEquals(CellState.EMPTY, s.board.get(Point(0, 1)))
        assertTrue(s.capturesByBlack >= 2)
    }

    @Test fun simpleKoBlocksImmediateRecapture() {
        // Classic ko shape on 9x9.
        var s = newGame()
        s = play(s, 2, 2) // B
        s = play(s, 2, 3) // W
        s = play(s, 3, 1) // B
        s = play(s, 3, 4) // W
        s = play(s, 4, 2) // B
        s = play(s, 4, 3) // W
        s = play(s, 3, 3) // B at center; W at (3,2) will be the ko stone
        s = play(s, 3, 2) // W captures B at (3,3)
        // Now black tries to immediately recapture at (3,3) -> ko violation.
        val res = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, Point(3, 3)))
        assertEquals(MoveResult.Rejected(MoveResult.Reason.KO_VIOLATION), res)
    }

    @Test fun doublePassEntersScoring() {
        var s = newGame()
        s = pass(s)
        s = pass(s)
        assertEquals(GameStatus.SCORING, s.status)
    }

    @Test fun resignEndsGame() {
        var s = newGame()
        val res = Rules.apply(s, MoveIntent(MoveType.RESIGN))
        assertTrue(res is MoveResult.Accepted)
        assertEquals(GameStatus.RESIGNED, (res as MoveResult.Accepted).newState.status)
    }

    @Test fun handicapPlacesBlackStones() {
        val s = GameState.newGame(GameConfig(boardSize = 9, handicap = 2))
        var blacks = 0
        for (r in 0 until 9) for (c in 0 until 9)
            if (s.board.get(Point(r, c)) == CellState.BLACK) blacks++
        assertEquals(2, blacks)
        assertEquals(StoneColor.WHITE, s.currentPlayer)
    }
}
