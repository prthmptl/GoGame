package com.weiqi.data

import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus

class SavedGameRepo(private val dao: SavedGameDao) {

    /** A single autosaved in-progress game; replaced as it advances. */
    private val currentId = "current"

    suspend fun saveCurrent(state: GameState) {
        if (state.status != GameStatus.ACTIVE && state.status != GameStatus.SCORING) return
        val now = System.currentTimeMillis()
        val existing = dao.get(currentId)
        dao.upsert(
            GameSerializer.toEntity(
                id = currentId,
                state = state,
                createdAt = existing?.createdAtMillis ?: now,
                updatedAt = now
            )
        )
    }

    suspend fun loadCurrent(): GameState? = dao.get(currentId)?.let { GameSerializer.fromEntity(it) }

    suspend fun clearCurrent() = dao.delete(currentId)

    suspend fun archiveCompleted(state: GameState) {
        val id = "game_${System.currentTimeMillis()}"
        val now = System.currentTimeMillis()
        dao.upsert(GameSerializer.toEntity(id, state, now, now))
    }

    suspend fun listAll(): List<SavedGameEntity> = dao.list()
}
