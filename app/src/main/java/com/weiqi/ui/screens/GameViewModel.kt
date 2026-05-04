package com.weiqi.ui.screens

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.weiqi.ai.BeginnerAI
import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.ScoreResult
import com.weiqi.engine.Scoring
import com.weiqi.engine.StoneColor
import com.weiqi.data.SavedGameRepo
import com.weiqi.sgf.Sgf
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

enum class Opponent { HUMAN, AI }

/** Default per-player main time (Fischer-ish: just main clock for v1). */
private const val DEFAULT_MAIN_MILLIS = 10L * 60L * 1000L

data class GameUi(
    val state: GameState,
    val rejection: String? = null,
    val deadStones: Set<Point> = emptySet(),
    val score: ScoreResult? = null,
    val aiThinking: Boolean = false,
    val opponent: Opponent = Opponent.HUMAN,
    val aiPlays: StoneColor = StoneColor.WHITE,
    val sgf: String? = null,
    val blackMillis: Long = DEFAULT_MAIN_MILLIS,
    val whiteMillis: Long = DEFAULT_MAIN_MILLIS,
    val timeoutLoser: StoneColor? = null
)

class GameViewModel(
    private val repo: SavedGameRepo? = null
) : ViewModel() {

    private val ai = BeginnerAI()
    private val _ui = MutableStateFlow(GameUi(state = GameState.newGame(GameConfig(boardSize = 9))))
    val ui: StateFlow<GameUi> = _ui.asStateFlow()

    private var clockJob: Job? = null
    private var lastTickMillis: Long = 0L

    private fun autosave() {
        val s = _ui.value.state
        if (repo == null) return
        viewModelScope.launch { repo.saveCurrent(s) }
    }

    private fun archiveAndClear() {
        val s = _ui.value.state
        if (repo == null) return
        viewModelScope.launch {
            repo.archiveCompleted(s)
            repo.clearCurrent()
        }
    }

    fun startGame(config: GameConfig, opponent: Opponent, aiPlays: StoneColor = StoneColor.WHITE) {
        _ui.value = GameUi(
            state = GameState.newGame(config),
            opponent = opponent,
            aiPlays = aiPlays,
            blackMillis = DEFAULT_MAIN_MILLIS,
            whiteMillis = DEFAULT_MAIN_MILLIS
        )
        startClock()
        maybeTriggerAi()
    }

    fun loadGame(state: GameState, opponent: Opponent = Opponent.HUMAN) {
        _ui.value = GameUi(state = state, opponent = opponent)
        startClock()
    }

    /** Restore the autosaved current game. Returns true if one was loaded. */
    suspend fun resumeCurrent(): Boolean {
        val state = repo?.loadCurrent() ?: return false
        if (state.status == GameStatus.COMPLETED || state.status == GameStatus.RESIGNED) return false
        _ui.value = GameUi(state = state, opponent = Opponent.HUMAN)
        startClock()
        return true
    }

    fun tap(point: Point) {
        val cur = _ui.value
        if (cur.state.status == GameStatus.SCORING) {
            toggleDead(point); return
        }
        play(MoveIntent(MoveType.PLACE_STONE, point))
    }

    fun pass() = play(MoveIntent(MoveType.PASS))
    fun resign() = play(MoveIntent(MoveType.RESIGN))

    fun undo() {
        val cur = _ui.value.state
        if (cur.history.isEmpty()) return
        val drop = if (_ui.value.opponent == Opponent.AI &&
            cur.history.lastOrNull()?.player == _ui.value.aiPlays) 2 else 1
        val newHistory = cur.history.dropLast(drop)
        var s = GameState.newGame(cur.config)
        for (m in newHistory) {
            val intent = when (m.type) {
                MoveType.PASS -> MoveIntent(MoveType.PASS)
                MoveType.RESIGN -> MoveIntent(MoveType.RESIGN)
                MoveType.PLACE_STONE -> MoveIntent(MoveType.PLACE_STONE, m.point)
            }
            val r = Rules.apply(s, intent)
            if (r is MoveResult.Accepted) s = r.newState
        }
        _ui.update { it.copy(state = s, rejection = null, score = null) }
    }

    private fun play(intent: MoveIntent) {
        val cur = _ui.value
        val res = Rules.apply(cur.state, intent)
        when (res) {
            is MoveResult.Rejected -> _ui.update { it.copy(rejection = humanizeReason(res.reason)) }
            is MoveResult.Accepted -> {
                _ui.update { it.copy(state = res.newState, rejection = null) }
                when (res.newState.status) {
                    GameStatus.SCORING -> { stopClock(); computeScore(); autosave() }
                    GameStatus.RESIGNED, GameStatus.COMPLETED -> { stopClock(); archiveAndClear() }
                    else -> { autosave(); maybeTriggerAi() }
                }
            }
        }
    }

    private fun maybeTriggerAi() {
        val cur = _ui.value
        if (cur.opponent != Opponent.AI) return
        if (cur.state.status != GameStatus.ACTIVE) return
        if (cur.state.currentPlayer != cur.aiPlays) return
        _ui.update { it.copy(aiThinking = true) }
        viewModelScope.launch {
            val intent = withContext(Dispatchers.Default) { ai.chooseMove(cur.state) }
            _ui.update { it.copy(aiThinking = false) }
            play(intent)
        }
    }

    fun toggleDead(p: Point) {
        val cur = _ui.value
        if (cur.state.status != GameStatus.SCORING) return
        if (cur.state.board.get(p) == com.weiqi.engine.CellState.EMPTY) return
        val next = if (p in cur.deadStones) cur.deadStones - p else cur.deadStones + p
        _ui.update { it.copy(deadStones = next) }
        computeScore()
    }

    fun confirmScore() {
        _ui.update {
            it.copy(
                state = it.state.copy(status = GameStatus.COMPLETED),
                sgf = Sgf.export(it.state, score = it.score)
            )
        }
        stopClock()
    }

    fun resumePlay() {
        _ui.update {
            it.copy(
                state = it.state.copy(status = GameStatus.ACTIVE, consecutivePasses = 0),
                deadStones = emptySet(),
                score = null
            )
        }
        startClock()
    }

    fun exportSgf(): String {
        val cur = _ui.value
        val sgf = Sgf.export(cur.state, score = cur.score)
        _ui.update { it.copy(sgf = sgf) }
        return sgf
    }

    private fun computeScore() {
        val cur = _ui.value
        val score = Scoring.score(cur.state, cur.deadStones)
        _ui.update { it.copy(score = score) }
    }

    private fun humanizeReason(r: MoveResult.Reason): String = when (r) {
        MoveResult.Reason.GAME_NOT_ACTIVE -> "Game is not active"
        MoveResult.Reason.OUT_OF_BOUNDS -> "Off the board"
        MoveResult.Reason.OCCUPIED -> "Point already occupied"
        MoveResult.Reason.SUICIDE -> "Suicide is not allowed"
        MoveResult.Reason.KO_VIOLATION -> "Ko: cannot retake immediately"
        MoveResult.Reason.SUPERKO_VIOLATION -> "Superko: position would repeat"
    }

    // ---- Clock ----

    private fun startClock() {
        stopClock()
        lastTickMillis = System.currentTimeMillis()
        clockJob = viewModelScope.launch {
            while (true) {
                delay(250)
                val now = System.currentTimeMillis()
                val delta = now - lastTickMillis
                lastTickMillis = now
                val cur = _ui.value
                if (cur.state.status != GameStatus.ACTIVE) continue
                val active = cur.state.currentPlayer
                _ui.update { st ->
                    val (b, w) = if (active == StoneColor.BLACK)
                        (st.blackMillis - delta).coerceAtLeast(0) to st.whiteMillis
                    else
                        st.blackMillis to (st.whiteMillis - delta).coerceAtLeast(0)
                    val timeoutLoser = when {
                        b == 0L && st.timeoutLoser == null -> StoneColor.BLACK
                        w == 0L && st.timeoutLoser == null -> StoneColor.WHITE
                        else -> st.timeoutLoser
                    }
                    val newStatus = if (timeoutLoser != null) GameStatus.COMPLETED else st.state.status
                    st.copy(
                        blackMillis = b,
                        whiteMillis = w,
                        timeoutLoser = timeoutLoser,
                        state = if (newStatus != st.state.status) st.state.copy(status = newStatus) else st.state
                    )
                }
            }
        }
    }

    private fun stopClock() {
        clockJob?.cancel(); clockJob = null
    }

    override fun onCleared() {
        stopClock()
        super.onCleared()
    }

    companion object {
        fun formatTime(millis: Long): String {
            val total = (millis / 1000).coerceAtLeast(0)
            val m = total / 60
            val s = total % 60
            return "%02d:%02d".format(m, s)
        }
    }
}
