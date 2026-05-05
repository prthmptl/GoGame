package com.weiqi.ai

import com.weiqi.engine.Board
import com.weiqi.engine.CellState
import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.Scoring
import com.weiqi.engine.StoneColor
import kotlin.random.Random

/**
 * Stronger heuristic AI:
 *   1. Capture moves: take the largest available capture.
 *   2. Defend: save own groups in atari (only if escape gives >= 2 liberties).
 *   3. Attack: place stones that put an opponent group into atari.
 *   4. Connection: connect own stones that share a single shared liberty.
 *   5. One-ply lookahead: avoid self-atari, prefer moves with stable liberties,
 *      and bias by Chinese score delta from a quick eval.
 *   6. Pass: when no positive-value move remains in late game.
 */
class IntermediateAI(private val random: Random = Random(0)) : GoAi {

    override fun chooseMove(state: GameState): MoveIntent {
        val legal = Rules.legalPlacements(state)
        if (legal.isEmpty()) return MoveIntent(MoveType.PASS)

        val player = state.currentPlayer
        val opponent = player.other()
        val board = state.board

        // 1. Largest capture wins.
        val captures = legal.mapNotNull { p ->
            val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            if (res is MoveResult.Accepted && res.move.captured.isNotEmpty()) p to res.move.captured.size else null
        }
        if (captures.isNotEmpty()) {
            val best = captures.maxByOrNull { it.second }!!.first
            return MoveIntent(MoveType.PLACE_STONE, best)
        }

        // 2. Save own groups in atari.
        val saves = legal.filter { savesAtari(state, it) }
        if (saves.isNotEmpty()) {
            return MoveIntent(MoveType.PLACE_STONE, pickByEval(saves, state))
        }

        // 3. Atari attacks.
        val attacks = legal.filter { putsOpponentInAtari(state, it, opponent) }
        if (attacks.isNotEmpty()) {
            return MoveIntent(MoveType.PLACE_STONE, pickByEval(attacks, state))
        }

        // 4-5. Score every legal move with a richer evaluation.
        val sizeSq = board.size * board.size
        val opening = state.moveNumber < board.size * 2
        val late = state.moveNumber > sizeSq

        val scored = legal.map { p ->
            val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            val score = if (res is MoveResult.Accepted) {
                evaluateMove(p, state, res.newState, player, opponent, opening)
            } else Int.MIN_VALUE
            p to score
        }

        val maxScore = scored.maxOf { it.second }
        if (maxScore == Int.MIN_VALUE) return MoveIntent(MoveType.PASS)

        // Pass in late game when no move helps.
        if (late && maxScore <= 0) return MoveIntent(MoveType.PASS)

        val top = scored.filter { it.second >= maxScore - 1 }.map { it.first }
        return MoveIntent(MoveType.PLACE_STONE, top.random(random))
    }

    private fun savesAtari(state: GameState, p: Point): Boolean {
        val board = state.board
        val player = state.currentPlayer
        val own = CellState.of(player)
        val anyAtari = board.neighbors(p).any { n ->
            board.get(n) == own && com.weiqi.engine.internalLiberties(board, n) == 1
        }
        if (!anyAtari) return false
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return false
        return com.weiqi.engine.internalLiberties(res.newState.board, p) >= 2
    }

    private fun putsOpponentInAtari(state: GameState, p: Point, opponent: StoneColor): Boolean {
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return false
        val nb = res.newState.board
        val opp = CellState.of(opponent)
        // Check any opponent stone adjacent to placed point now sits in a 1-liberty group.
        return nb.neighbors(p).any { n ->
            nb.get(n) == opp && com.weiqi.engine.internalLiberties(nb, n) == 1
        }
    }

    private fun pickByEval(candidates: List<Point>, state: GameState): Point {
        val player = state.currentPlayer
        val opponent = player.other()
        return candidates.maxByOrNull { p ->
            val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            if (res is MoveResult.Accepted) {
                evaluateMove(p, state, res.newState, player, opponent, opening = false)
            } else Int.MIN_VALUE
        }!!
    }

    private fun evaluateMove(
        p: Point,
        before: GameState,
        after: GameState,
        player: StoneColor,
        opponent: StoneColor,
        opening: Boolean
    ): Int {
        val board = after.board
        val size = board.size
        var score = 0

        // Penalty: self-atari.
        val ownLibs = com.weiqi.engine.internalLiberties(board, p)
        when (ownLibs) {
            1 -> score -= 30
            2 -> score -= 4
            3 -> score += 0
            else -> score += 2
        }

        // Bonus: pressure on adjacent opponent groups.
        val opp = CellState.of(opponent)
        for (n in board.neighbors(p)) {
            if (board.get(n) == opp) {
                val libs = com.weiqi.engine.internalLiberties(board, n)
                score += when (libs) {
                    1 -> 12
                    2 -> 5
                    3 -> 2
                    else -> 0
                }
            }
        }

        // Connection bonus: hugging own stones.
        val own = CellState.of(player)
        val ownAdj = board.neighbors(p).count { board.get(it) == own }
        score += ownAdj

        // Edge / line preference.
        val edgeDist = minOf(p.row, p.col, size - 1 - p.row, size - 1 - p.col)
        score += if (opening) {
            when (edgeDist) { 0 -> -5; 1 -> -2; 2 -> 3; 3 -> 3; else -> 2 }
        } else {
            when (edgeDist) { 0 -> -2; 1 -> 0; else -> 1 }
        }

        // Quick territory delta — runs scoring on cheap boards (mainly meaningful on 9×9).
        if (size <= 13) {
            val before = Scoring.score(before).margin
            val after = Scoring.score(after).margin
            val delta = after - before
            // From player's perspective (positive = good for player).
            val perPlayer = if (player == StoneColor.BLACK) delta else -delta
            score += (perPlayer * 2).toInt().coerceIn(-15, 15)
        }

        score += random.nextInt(0, 2)
        return score
    }
}
