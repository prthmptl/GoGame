package com.weiqi.puzzle

import com.weiqi.engine.Board
import com.weiqi.engine.CellState
import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.StoneColor

/** Difficulty tier — used for grouping in the UI. */
enum class PuzzleTier(val label: String, val description: String) {
    FUNDAMENTALS("Fundamentals", "Essential shapes and reading."),
    INTERMEDIATE("Intermediate", "Tactical fights and sacrifices."),
    ADVANCED("Advanced", "Deep reading and tesuji.")
}

/**
 * A single tsumego problem.
 *
 * The user plays [toPlay]. The puzzle ships with a "main line": user move, opponent
 * response, user move, … the puzzle is solved when the user has played the last
 * user move from the main line. Wrong moves reset to the previous correct state
 * and increment the mistake counter.
 */
data class Puzzle(
    val id: String,
    val title: String,
    val tier: PuzzleTier,
    val theme: String,
    val description: String,
    val hint: String,
    val boardSize: Int,
    val setupBlack: List<Point>,
    val setupWhite: List<Point>,
    val toPlay: StoneColor,
    /** Alternating moves: user, opponent, user, opponent, … always starting with user. */
    val mainLine: List<Point>
) {
    /** Build the initial GameState with the prepared stones. */
    fun initialState(): GameState {
        val cfg = GameConfig(boardSize = boardSize, handicap = 0, komi = 0.0)
        var s = GameState.newGame(cfg)
        var board: Board = s.board
        for (p in setupBlack) board = board.set(p, CellState.BLACK)
        for (p in setupWhite) board = board.set(p, CellState.WHITE)
        s = s.copy(
            board = board,
            currentPlayer = toPlay,
            previousHashes = setOf(board.zobristHash())
        )
        return s
    }

    /** Index in [mainLine] of the user's required moves: 0, 2, 4, … */
    val totalUserMoves: Int get() = (mainLine.size + 1) / 2
}

/** State of a puzzle attempt. */
data class PuzzleSession(
    val puzzle: Puzzle,
    val state: GameState,
    val nextMoveIndex: Int = 0,   // index into mainLine — next move expected
    val mistakes: Int = 0,
    val solved: Boolean = false,
    val message: String? = null,
    val showHint: Boolean = false
) {
    val userMovesPlayed: Int get() = (nextMoveIndex + 1) / 2
    val progressLabel: String get() = "${userMovesPlayed.coerceAtMost(puzzle.totalUserMoves)}/${puzzle.totalUserMoves}"
}

/** Pure logic for advancing a puzzle session. */
object PuzzleEngine {
    fun start(puzzle: Puzzle): PuzzleSession =
        PuzzleSession(puzzle = puzzle, state = puzzle.initialState())

    fun reset(session: PuzzleSession): PuzzleSession =
        PuzzleSession(puzzle = session.puzzle, state = session.puzzle.initialState())

    fun toggleHint(session: PuzzleSession): PuzzleSession =
        session.copy(showHint = !session.showHint)

    /**
     * Apply a user tap. Returns the next session — possibly with a "wrong move" message
     * and the same state, or with the move + opponent response applied.
     */
    fun userTap(session: PuzzleSession, point: Point): PuzzleSession {
        if (session.solved) return session
        val expected = session.puzzle.mainLine.getOrNull(session.nextMoveIndex)
            ?: return session
        if (point != expected) {
            // Validate the tap is at least legal so we give a smarter message.
            val res = Rules.apply(session.state, MoveIntent(MoveType.PLACE_STONE, point))
            val msg = when (res) {
                is MoveResult.Rejected -> "Illegal there. Try again."
                is MoveResult.Accepted -> "Not the right move. Try again."
            }
            return session.copy(mistakes = session.mistakes + 1, message = msg)
        }

        // Play the user move.
        var s = session.state
        val userRes = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, expected))
        if (userRes !is MoveResult.Accepted) {
            return session.copy(message = "Illegal puzzle move (data bug).")
        }
        s = userRes.newState

        var newIndex = session.nextMoveIndex + 1

        // If a scripted opponent response remains, play it.
        val oppMove = session.puzzle.mainLine.getOrNull(newIndex)
        if (oppMove != null) {
            val oppRes = Rules.apply(s, MoveIntent(MoveType.PLACE_STONE, oppMove))
            if (oppRes is MoveResult.Accepted) {
                s = oppRes.newState
                newIndex += 1
            }
        }

        val solved = newIndex >= session.puzzle.mainLine.size
        return session.copy(
            state = s,
            nextMoveIndex = newIndex,
            solved = solved,
            message = if (solved) "Solved!" else "Good. Keep going.",
            showHint = false
        )
    }
}

object PuzzleLibrary {
    /**
     * Hand-curated puzzles. All boards are 9×9 to keep solutions readable.
     * Coordinates are (row, col) starting at top-left.
     */
    val all: List<Puzzle> = listOf(
        // ---- Fundamentals ----
        Puzzle(
            id = "f1_capture",
            title = "First Capture",
            tier = PuzzleTier.FUNDAMENTALS,
            theme = "Capture",
            description = "Black to play. Take the marked stone.",
            hint = "The white stone has only one liberty. Take it.",
            boardSize = 9,
            setupWhite = listOf(Point(4, 4)),
            setupBlack = listOf(Point(3, 4), Point(5, 4), Point(4, 3)),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(4, 5))
        ),
        Puzzle(
            id = "f2_atari",
            title = "Atari!",
            tier = PuzzleTier.FUNDAMENTALS,
            theme = "Atari",
            description = "Black to play. Put the white pair into atari.",
            hint = "Block the only escape route on the side.",
            boardSize = 9,
            setupWhite = listOf(Point(4, 4), Point(4, 5)),
            setupBlack = listOf(Point(3, 4), Point(3, 5), Point(5, 4), Point(5, 5), Point(4, 3)),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(4, 6))
        ),
        Puzzle(
            id = "f3_save",
            title = "Save Yourself",
            tier = PuzzleTier.FUNDAMENTALS,
            theme = "Atari escape",
            description = "Black to play. The black stone is in atari — escape.",
            hint = "Extend toward open space, not toward more white stones.",
            boardSize = 9,
            setupBlack = listOf(Point(4, 4)),
            setupWhite = listOf(Point(3, 4), Point(4, 5), Point(5, 4)),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(4, 3))
        ),
        // ---- Intermediate ----
        Puzzle(
            id = "i1_net",
            title = "The Net",
            tier = PuzzleTier.INTERMEDIATE,
            theme = "Trap",
            description = "Black to play. Catch the white stone — it has two ways to run.",
            hint = "A net (geta) is a knight's-move-style cap that takes both escape squares.",
            boardSize = 9,
            setupWhite = listOf(Point(4, 4)),
            setupBlack = listOf(Point(3, 4), Point(4, 3), Point(5, 3)),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(5, 5))
        ),
        Puzzle(
            id = "i2_double_atari",
            title = "Double Atari",
            tier = PuzzleTier.INTERMEDIATE,
            theme = "Tesuji",
            description = "Black to play. One move puts both white stones in atari at once.",
            hint = "Find the point between the two whites that keeps your own stone alive.",
            boardSize = 9,
            setupWhite = listOf(Point(2, 3), Point(4, 3)),
            setupBlack = listOf(
                Point(2, 2), Point(2, 4),
                Point(4, 2), Point(4, 4)
            ),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(3, 3))
        ),
        Puzzle(
            id = "i3_connect",
            title = "Make the Connection",
            tier = PuzzleTier.INTERMEDIATE,
            theme = "Connection",
            description = "Black to play. Link the two black groups before White cuts.",
            hint = "There is exactly one point that connects both groups.",
            boardSize = 9,
            setupBlack = listOf(Point(4, 2), Point(4, 4)),
            setupWhite = listOf(Point(3, 3), Point(5, 3)),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(4, 3))
        ),
        // ---- Advanced ----
        Puzzle(
            id = "a1_two_eyes",
            title = "Make Two Eyes",
            tier = PuzzleTier.ADVANCED,
            theme = "Life",
            description = "Black to play and live.",
            hint = "There is a vital point that splits one big eye into two.",
            boardSize = 9,
            setupBlack = listOf(
                Point(0, 1), Point(0, 2), Point(0, 3), Point(0, 4),
                Point(1, 0), Point(1, 4),
                Point(2, 0), Point(2, 4),
                Point(3, 0), Point(3, 1), Point(3, 2), Point(3, 3), Point(3, 4)
            ),
            setupWhite = emptyList(),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(1, 2))
        ),
        Puzzle(
            id = "a2_kill",
            title = "Kill the Group",
            tier = PuzzleTier.ADVANCED,
            theme = "Death",
            description = "Black to play. Take away White's second eye.",
            hint = "Find the vital point inside White's shape.",
            boardSize = 9,
            setupWhite = listOf(
                Point(0, 1), Point(0, 2), Point(0, 3), Point(0, 4),
                Point(1, 0), Point(1, 4),
                Point(2, 0), Point(2, 4),
                Point(3, 0), Point(3, 1), Point(3, 2), Point(3, 3), Point(3, 4)
            ),
            setupBlack = listOf(
                Point(4, 0), Point(4, 1), Point(4, 2), Point(4, 3), Point(4, 4)
            ),
            toPlay = StoneColor.BLACK,
            mainLine = listOf(Point(1, 2))
        )
    )

    val byTier: Map<PuzzleTier, List<Puzzle>> = all.groupBy { it.tier }
}

/** Whether the session's game state has terminated (rare in puzzles, but defensive). */
fun PuzzleSession.isTerminal(): Boolean = state.status != GameStatus.ACTIVE
