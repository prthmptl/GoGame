package com.weiqi.ai

import com.weiqi.engine.Board
import com.weiqi.engine.CellState
import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.StoneColor
import kotlin.random.Random

/**
 * Heuristic Go AI. Not strong; designed to give a beginner a reasonable game.
 *
 * Move-selection priorities (highest first):
 *   1. Capture an opponent group in atari.
 *   2. Save own group in atari (if the rescuing move yields >1 liberty).
 *   3. Play near existing stones, with a slight preference for the 3rd/4th line early.
 *   4. Pass when no profitable moves remain.
 */
class BeginnerAI(private val random: Random = Random.Default) : GoAi {

    override fun chooseMove(state: GameState): MoveIntent {
        val legal = Rules.legalPlacements(state)
        if (legal.isEmpty()) return MoveIntent(MoveType.PASS)

        val player = state.currentPlayer
        val opponent = player.other()
        val board = state.board

        val captures = legal.filter { p ->
            val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            res is MoveResult.Accepted && res.move.captured.isNotEmpty()
        }
        if (captures.isNotEmpty()) return MoveIntent(MoveType.PLACE_STONE, pickBest(captures, board, player))

        val saves = legal.filter { p -> savesAtari(state, p) }
        if (saves.isNotEmpty()) return MoveIntent(MoveType.PLACE_STONE, pickBest(saves, board, player))

        val scored = legal.map { p -> p to scoreMove(p, board, player, opponent, state.moveNumber) }
        val maxScore = scored.maxOf { it.second }
        val top = scored.filter { it.second >= maxScore - 1 }.map { it.first }
        if (top.isEmpty()) return MoveIntent(MoveType.PASS)

        if (state.moveNumber > board.size * board.size && maxScore <= 0) {
            return MoveIntent(MoveType.PASS)
        }
        return MoveIntent(MoveType.PLACE_STONE, top.random(random))
    }

    private fun pickBest(candidates: List<Point>, board: Board, player: StoneColor): Point {
        return candidates.maxByOrNull { scoreMove(it, board, player, player.other(), 0) }!!
    }

    private fun savesAtari(state: GameState, p: Point): Boolean {
        val board = state.board
        val player = state.currentPlayer
        val ownState = CellState.of(player)
        val atariNeighbors = board.neighbors(p).any { n ->
            board.get(n) == ownState &&
                com.weiqi.engine.internalLiberties(board, n) == 1
        }
        if (!atariNeighbors) return false
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return false
        return com.weiqi.engine.internalLiberties(res.newState.board, p) >= 2
    }

    private fun scoreMove(
        p: Point,
        board: Board,
        player: StoneColor,
        opponent: StoneColor,
        moveNumber: Int
    ): Int {
        var score = 0
        val size = board.size
        val ownState = CellState.of(player)
        val oppState = CellState.of(opponent)

        val edgeDist = minOf(p.row, p.col, size - 1 - p.row, size - 1 - p.col)
        score += if (moveNumber < size * 2) {
            when (edgeDist) {
                0 -> -3
                1 -> -1
                2, 3 -> 2
                else -> 1
            }
        } else 0

        for (n in board.neighbors(p)) {
            val st = board.get(n)
            when (st) {
                ownState -> score += 1
                oppState -> score += 2
                else -> {}
            }
        }

        score += random.nextInt(0, 2)
        return score
    }
}
