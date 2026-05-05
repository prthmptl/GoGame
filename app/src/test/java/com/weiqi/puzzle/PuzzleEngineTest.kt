package com.weiqi.puzzle

import com.weiqi.engine.CellState
import com.weiqi.engine.GameStatus
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class PuzzleEngineTest {

    @Test fun bundledPuzzleIdsAreUnique() {
        val ids = PuzzleLibrary.all.map { it.id }
        assertEquals(ids.size, ids.toSet().size)
    }

    @Test fun bundledPuzzlesHaveValidSetup() {
        PuzzleLibrary.all.forEach { puzzle ->
            assertTrue("empty main line: ${puzzle.id}", puzzle.mainLine.isNotEmpty())
            assertTrue("bad board size: ${puzzle.id}", puzzle.boardSize > 1)

            val black = puzzle.setupBlack.toSet()
            val white = puzzle.setupWhite.toSet()
            assertTrue("overlapping setup stones: ${puzzle.id}", black.intersect(white).isEmpty())

            (black + white + puzzle.mainLine).forEach { p ->
                assertTrue("point out of bounds in ${puzzle.id}: $p", p.row in 0 until puzzle.boardSize)
                assertTrue("point out of bounds in ${puzzle.id}: $p", p.col in 0 until puzzle.boardSize)
            }

            val state = puzzle.initialState()
            assertEquals(GameStatus.ACTIVE, state.status)
            assertEquals(puzzle.toPlay, state.currentPlayer)
            black.forEach { assertEquals(CellState.BLACK, state.board.get(it)) }
            white.forEach { assertEquals(CellState.WHITE, state.board.get(it)) }
        }
    }

    @Test fun bundledPuzzlesSolveThroughMainLine() {
        PuzzleLibrary.all.forEach { puzzle ->
            var session = PuzzleEngine.start(puzzle)
            while (!session.solved) {
                val expected = puzzle.mainLine[session.nextMoveIndex]
                val beforeMistakes = session.mistakes
                session = PuzzleEngine.userTap(session, expected)

                assertEquals("unexpected mistake in ${puzzle.id}", beforeMistakes, session.mistakes)
                assertNotEquals("illegal puzzle data in ${puzzle.id}", "Illegal puzzle move (data bug).", session.message)
            }

            assertTrue("not solved: ${puzzle.id}", session.solved)
            assertEquals("Solved!", session.message)
        }
    }

    @Test fun wrongMoveKeepsBoardAndCountsMistake() {
        val puzzle = PuzzleLibrary.all.first()
        val session = PuzzleEngine.start(puzzle)
        val wrong = (0 until puzzle.boardSize)
            .flatMap { row -> (0 until puzzle.boardSize).map { col -> com.weiqi.engine.Point(row, col) } }
            .first { it != puzzle.mainLine.first() && session.state.board.get(it) == CellState.EMPTY }

        val next = PuzzleEngine.userTap(session, wrong)

        assertFalse(next.solved)
        assertEquals(1, next.mistakes)
        assertEquals(session.state.board, next.state.board)
    }
}
