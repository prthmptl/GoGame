package com.weiqi.data

import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus
import com.weiqi.engine.StoneColor

class SavedGameRepo(private val dao: SavedGameDao) {

    /** A single autosaved in-progress game; replaced as it advances. */
    private val currentId = "current"

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
        resultLabel: String
    ) {
        val id = "game_${System.currentTimeMillis()}"
        val now = System.currentTimeMillis()
        dao.upsert(
            GameSerializer.toEntity(
                id = id,
                state = state,
                createdAt = now,
                updatedAt = now,
                opponentLabel = opponentLabel,
                resultLabel = resultLabel,
                youColor = youColor.name
            )
        )
    }

    suspend fun listAll(): List<SavedGameEntity> = dao.list()
    suspend fun listCompleted(limit: Int = 10): List<SavedGameEntity> = dao.listCompleted(limit)
}
