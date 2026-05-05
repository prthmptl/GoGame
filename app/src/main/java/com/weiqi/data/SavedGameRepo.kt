package com.weiqi.data

import android.content.Context
import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus
import com.weiqi.engine.ScoreResult
import com.weiqi.engine.StoneColor
import com.weiqi.sgf.Sgf
import java.io.File

class SavedGameRepo(
    private val dao: SavedGameDao,
    private val context: Context? = null
) {

    /** A single autosaved in-progress game; replaced as it advances. */
    private val currentId = "current"

    private fun sgfDir(): File? = context?.let {
        File(it.filesDir, "sgf").apply { mkdirs() }
    }

    suspend fun saveCurrent(
        state: GameState,
        opponentLabel: String,
        youColor: StoneColor
    ) {
        if (state.status != GameStatus.ACTIVE && state.status != GameStatus.SCORING) return
        val now = System.currentTimeMillis()
        val existing = dao.get(currentId)
        dao.upsert(
            GameSerializer.toEntity(
                id = currentId,
                state = state,
                createdAt = existing?.createdAtMillis ?: now,
                updatedAt = now,
                opponentLabel = opponentLabel,
                resultLabel = "",
                youColor = youColor.name
            )
        )
    }

    suspend fun loadCurrent(): GameState? = dao.get(currentId)?.let { GameSerializer.fromEntity(it) }

    suspend fun clearCurrent() = dao.delete(currentId)

    suspend fun archiveCompleted(
        state: GameState,
        opponentLabel: String,
        youColor: StoneColor,
        resultLabel: String,
        score: ScoreResult? = null
    ): String {
        val id = "game_${System.currentTimeMillis()}"
        val now = System.currentTimeMillis()
        val sgfText = Sgf.export(
            state = state,
            score = score,
            blackName = if (youColor == StoneColor.BLACK) "You" else opponentLabel,
            whiteName = if (youColor == StoneColor.WHITE) "You" else opponentLabel
        )
        val path = sgfDir()?.let { dir ->
            val f = File(dir, "$id.sgf")
            f.writeText(sgfText)
            f.absolutePath
        } ?: ""
        dao.upsert(
            GameSerializer.toEntity(
                id = id,
                state = state,
                createdAt = now,
                updatedAt = now,
                opponentLabel = opponentLabel,
                resultLabel = resultLabel,
                youColor = youColor.name,
                sgfPath = path
            )
        )
        return path
    }

    suspend fun delete(id: String) {
        dao.get(id)?.sgfPath?.takeIf { it.isNotBlank() }?.let { File(it).delete() }
        dao.delete(id)
    }

    suspend fun listAll(): List<SavedGameEntity> = dao.list()
    suspend fun listCompleted(limit: Int = 10): List<SavedGameEntity> = dao.listCompleted(limit)
    suspend fun get(id: String): SavedGameEntity? = dao.get(id)
}
