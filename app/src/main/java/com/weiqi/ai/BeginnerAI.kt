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
import com.weiqi.engine.internalLiberties
import kotlin.random.Random

/**
 * Heuristic Go AI. Move-selection priorities (highest first):
 *   1. Capture an opponent group in atari.
 *   2. Save own group in atari (extend OR capture the attacker).
 *   3. Avoid self-atari, never fill own eye.
 *   4. Top-N candidates evaluated 1 ply deep against opponent's best capture/atari.
 *   5. Local response bias near opponent's last move.
 *   6. Empty-board opening on a star point.
 */
class BeginnerAI(private val random: Random = Random.Default) : GoAi {

    override fun chooseMove(state: GameState): MoveIntent {
        val board = state.board

        // (6) Opening on an empty board: star point.
        if (state.history.isEmpty()) {
            starPoint(board.size)?.let { return MoveIntent(MoveType.PLACE_STONE, it) }
        }

        val legal = Rules.legalPlacements(state).filterNot { isOwnEye(board, it, state.currentPlayer) }
        if (legal.isEmpty()) return MoveIntent(MoveType.PASS)

        val player = state.currentPlayer
        val opponent = player.other()

        // (1) Captures.
        val captures = legal.mapNotNull { p ->
            val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            if (res is MoveResult.Accepted && res.move.captured.isNotEmpty()) p to res else null
        }
        if (captures.isNotEmpty()) {
            return MoveIntent(MoveType.PLACE_STONE, captures.maxBy { (p, res) ->
                res.move.captured.size * 10 + scoreMove(p, board, player, opponent, state.moveNumber, state.lastMove?.point)
            }.first)
        }

        // (2) Escape atari: extend OR capture the attacker.
        val saves = legal.filter { savesAtari(state, it) }
        if (saves.isNotEmpty()) {
            return MoveIntent(MoveType.PLACE_STONE, pickBest(saves, board, player, state))
        }

        // (3) Drop self-atari moves unless they capture (already handled above).
        val safe = legal.filter { !isSelfAtari(state, it) }
        val pool = safe.ifEmpty { legal }

        // Heuristic pre-score, then 1-ply lookahead on the top N.
        val scored = pool.map { p ->
            p to scoreMove(p, board, player, opponent, state.moveNumber, state.lastMove?.point)
        }
        val maxScore = scored.maxOf { it.second }
        val topPool = scored.filter { it.second >= maxScore - 2 }.map { it.first }
        val topN = topPool.shuffled(random).take(8)

        val refined = topN.map { p -> p to lookahead1Ply(state, p) }
        val best = refined.maxOf { it.second }
        val finalists = refined.filter { it.second >= best - 1 }.map { it.first }

        if (state.moveNumber > board.size * board.size && maxScore <= 0 && best <= 0) {
            return MoveIntent(MoveType.PASS)
        }
        return MoveIntent(MoveType.PLACE_STONE, finalists.random(random))
    }

    private fun pickBest(candidates: List<Point>, board: Board, player: StoneColor, state: GameState): Point {
        val last = state.lastMove?.point
        return candidates.maxBy { scoreMove(it, board, player, player.other(), state.moveNumber, last) }
    }

    /** True if [p] saves an own group currently in atari, by extending OR by capturing the attacker. */
    private fun savesAtari(state: GameState, p: Point): Boolean {
        val board = state.board
        val player = state.currentPlayer
        val ownState = CellState.of(player)
        val ownInAtari = board.neighbors(p).any { n ->
            board.get(n) == ownState && internalLiberties(board, n) == 1
        }
        if (!ownInAtari) return false
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return false
        // Either it captured the attacker, or the resulting own group has > 1 liberty.
        if (res.move.captured.isNotEmpty()) return true
        return internalLiberties(res.newState.board, p) >= 2
    }

    /** Move that puts our own freshly-formed group at exactly 1 liberty (and captures nothing). */
    private fun isSelfAtari(state: GameState, p: Point): Boolean {
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return false
        if (res.move.captured.isNotEmpty()) return false
        return internalLiberties(res.newState.board, p) <= 1
    }

    /** True if [p] is fully surrounded by our own color (treating board edges as friendly). */
    private fun isOwnEye(board: Board, p: Point, player: StoneColor): Boolean {
        if (board.get(p) != CellState.EMPTY) return false
        val own = CellState.of(player)
        val neighbors = board.neighbors(p)
        if (neighbors.any { board.get(it) != own }) return false
        // Check diagonals: at most one diagonal may be hostile/empty (true eye approximation).
        var bad = 0
        val size = board.size
        for ((dr, dc) in arrayOf(-1 to -1, -1 to 1, 1 to -1, 1 to 1)) {
            val nr = p.row + dr; val nc = p.col + dc
            if (nr !in 0 until size || nc !in 0 until size) continue
            if (board.get(Point(nr, nc)) != own) bad++
        }
        // On the edge, no diagonals may be bad. In the middle, one is tolerated.
        val onEdge = p.row == 0 || p.col == 0 || p.row == size - 1 || p.col == size - 1
        return if (onEdge) bad == 0 else bad <= 1
    }

    /**
     * Play [p], let the opponent pick their best reply (capture > atari > nothing),
     * and return a score reflecting net stones / liberties after that exchange.
     */
    private fun lookahead1Ply(state: GameState, p: Point): Int {
        val res = Rules.apply(state, MoveIntent(MoveType.PLACE_STONE, p))
        if (res !is MoveResult.Accepted) return Int.MIN_VALUE
        val after = res.newState
        var score = res.move.captured.size * 10
        score += scoreMove(p, state.board, state.currentPlayer, state.currentPlayer.other(),
            state.moveNumber, state.lastMove?.point)

        val oppLegal = Rules.legalPlacements(after).take(40)
        var worst = 0
        for (op in oppLegal) {
            val r = Rules.apply(after, MoveIntent(MoveType.PLACE_STONE, op))
            if (r !is MoveResult.Accepted) continue
            val capturedUs = r.move.captured.size
            if (capturedUs > 0 && capturedUs * -10 < worst) worst = capturedUs * -10
            // Did opponent put our just-played stone in atari?
            val ourLibs = internalLiberties(r.newState.board, p).takeIf { r.newState.board.get(p) != CellState.EMPTY } ?: 0
            if (ourLibs == 1 && worst > -2) worst = -2
        }
        return score + worst
    }

    private fun scoreMove(
        p: Point,
        board: Board,
        player: StoneColor,
        opponent: StoneColor,
        moveNumber: Int,
        lastOpponentMove: Point?
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
            when (board.get(n)) {
                ownState -> score += 1
                oppState -> score += 2
                else -> {}
            }
        }

        // (5) Local response bias: prefer moves near opponent's last stone.
        if (lastOpponentMove != null) {
            val d = kotlin.math.abs(p.row - lastOpponentMove.row) +
                kotlin.math.abs(p.col - lastOpponentMove.col)
            score += when {
                d <= 2 -> 3
                d <= 4 -> 1
                else -> 0
            }
        }

        score += random.nextInt(0, 2)
        return score
    }

    private fun starPoint(size: Int): Point? = when (size) {
        9 -> Point(4, 4)
        13 -> Point(3, 3)
        19 -> Point(3, 3)
        else -> null
    }
}
