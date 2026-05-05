package com.weiqi.puzzle

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/** Persists which puzzles have been solved. */
class PuzzleProgress private constructor(prefs: SharedPreferences) {
    private val sp: SharedPreferences = prefs
    private val _solved = MutableStateFlow(load())
    val solved: StateFlow<Set<String>> = _solved

    private fun load(): Set<String> =
        sp.getStringSet(KEY, emptySet())?.toSet() ?: emptySet()

    fun isSolved(id: String): Boolean = id in _solved.value

    fun markSolved(id: String) {
        if (id in _solved.value) return
        val next = _solved.value + id
        sp.edit().putStringSet(KEY, next).apply()
        _solved.value = next
    }

    companion object {
        private const val KEY = "solved_puzzles"
        @Volatile private var instance: PuzzleProgress? = null
        fun get(context: Context): PuzzleProgress = instance ?: synchronized(this) {
            instance ?: PuzzleProgress(
                context.applicationContext.getSharedPreferences("weiqi.puzzles", Context.MODE_PRIVATE)
            ).also { instance = it }
        }
    }
}
