package com.weiqi.data

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

enum class BoardTheme { CLASSIC_WOOD, MINIMAL_PAPER, DARK_SLATE, HIGH_CONTRAST }

data class AppSettings(
    val boardTheme: BoardTheme = BoardTheme.CLASSIC_WOOD,
    val showCoordinates: Boolean = false,
    val showMoveNumbers: Boolean = false,
    val beginnerHints: Boolean = true
)

class SettingsStore private constructor(prefs: SharedPreferences) {
    private val sp: SharedPreferences = prefs
    private val _state = MutableStateFlow(load())
    val state: StateFlow<AppSettings> = _state

    private fun load(): AppSettings = AppSettings(
        boardTheme = runCatching {
            BoardTheme.valueOf(sp.getString("theme", BoardTheme.CLASSIC_WOOD.name)!!)
        }.getOrDefault(BoardTheme.CLASSIC_WOOD),
        showCoordinates = sp.getBoolean("coords", false),
        showMoveNumbers = sp.getBoolean("moveNumbers", false),
        beginnerHints = sp.getBoolean("hints", true)
    )

    fun update(transform: (AppSettings) -> AppSettings) {
        val next = transform(_state.value)
        sp.edit()
            .putString("theme", next.boardTheme.name)
            .putBoolean("coords", next.showCoordinates)
            .putBoolean("moveNumbers", next.showMoveNumbers)
            .putBoolean("hints", next.beginnerHints)
            .apply()
        _state.value = next
    }

    companion object {
        @Volatile private var instance: SettingsStore? = null
        fun get(context: Context): SettingsStore = instance ?: synchronized(this) {
            instance ?: SettingsStore(
                context.applicationContext.getSharedPreferences("weiqi.settings", Context.MODE_PRIVATE)
            ).also { instance = it }
        }
    }
}
