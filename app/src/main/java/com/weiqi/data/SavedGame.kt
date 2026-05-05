package com.weiqi.data

import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import android.content.Context
import com.weiqi.engine.GameConfig
import com.weiqi.engine.GameState
import com.weiqi.engine.GameStatus
import com.weiqi.engine.MoveIntent
import com.weiqi.engine.MoveResult
import com.weiqi.engine.MoveType
import com.weiqi.engine.Point
import com.weiqi.engine.Rules
import com.weiqi.engine.Ruleset

@Entity(tableName = "saved_games")
data class SavedGameEntity(
    @PrimaryKey val id: String,
    val createdAtMillis: Long,
    val updatedAtMillis: Long,
    val boardSize: Int,
    val komi: Double,
    val handicap: Int,
    val status: String,
    /** Encoded as a sequence of moves: e.g. "B,4,4|W,3,3|B,P|W,R". */
    val movesEncoded: String,
    @ColumnInfo(defaultValue = "Local") val opponentLabel: String = "Local",
    @ColumnInfo(defaultValue = "") val resultLabel: String = "",
    @ColumnInfo(defaultValue = "BLACK") val youColor: String = "BLACK"
)

@Dao
interface SavedGameDao {
    @Query("SELECT * FROM saved_games ORDER BY updatedAtMillis DESC")
    suspend fun list(): List<SavedGameEntity>

    @Query("SELECT * FROM saved_games WHERE status IN ('COMPLETED','RESIGNED') ORDER BY updatedAtMillis DESC LIMIT :limit")
    suspend fun listCompleted(limit: Int = 10): List<SavedGameEntity>

    @Query("SELECT * FROM saved_games WHERE id = :id")
    suspend fun get(id: String): SavedGameEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(game: SavedGameEntity)

    @Query("DELETE FROM saved_games WHERE id = :id")
    suspend fun delete(id: String)
}

@Database(entities = [SavedGameEntity::class], version = 2, exportSchema = false)
abstract class WeiqiDatabase : RoomDatabase() {
    abstract fun savedGames(): SavedGameDao

    companion object {
        @Volatile private var instance: WeiqiDatabase? = null
        fun get(context: Context): WeiqiDatabase = instance ?: synchronized(this) {
            instance ?: Room.databaseBuilder(
                context.applicationContext, WeiqiDatabase::class.java, "weiqi.db"
            )
                .fallbackToDestructiveMigration()
                .build()
                .also { instance = it }
        }
    }
}

object GameSerializer {
    fun encode(state: GameState): String = state.history.joinToString("|") { m ->
        when (m.type) {
            MoveType.PASS -> "${m.player.name[0]},P"
            MoveType.RESIGN -> "${m.player.name[0]},R"
            MoveType.PLACE_STONE -> "${m.player.name[0]},${m.point!!.row},${m.point.col}"
        }
    }

    /** Replay encoded moves on top of a fresh game with the same config. */
    fun decode(config: GameConfig, encoded: String): GameState {
        var s = GameState.newGame(config)
        if (encoded.isBlank()) return s
        for (token in encoded.split('|')) {
            val parts = token.split(',')
            val intent = when {
                parts.size == 2 && parts[1] == "P" -> MoveIntent(MoveType.PASS)
                parts.size == 2 && parts[1] == "R" -> MoveIntent(MoveType.RESIGN)
                parts.size == 3 -> MoveIntent(MoveType.PLACE_STONE, Point(parts[1].toInt(), parts[2].toInt()))
                else -> continue
            }
            val res = Rules.apply(s, intent)
            if (res is MoveResult.Accepted) s = res.newState else return s
        }
        return s
    }

    fun toEntity(
        id: String,
        state: GameState,
        createdAt: Long,
        updatedAt: Long,
        opponentLabel: String = "Local",
        resultLabel: String = "",
        youColor: String = "BLACK"
    ): SavedGameEntity =
        SavedGameEntity(
            id = id,
            createdAtMillis = createdAt,
            updatedAtMillis = updatedAt,
            boardSize = state.config.boardSize,
            komi = state.config.komi,
            handicap = state.config.handicap,
            status = state.status.name,
            movesEncoded = encode(state),
            opponentLabel = opponentLabel,
            resultLabel = resultLabel,
            youColor = youColor
        )

    fun fromEntity(e: SavedGameEntity): GameState {
        val cfg = GameConfig(boardSize = e.boardSize, ruleset = Ruleset.CHINESE, komi = e.komi, handicap = e.handicap)
        var s = decode(cfg, e.movesEncoded)
        if (s.status == GameStatus.ACTIVE && e.status == GameStatus.SCORING.name) {
            s = s.copy(status = GameStatus.SCORING)
        }
        return s
    }
}
