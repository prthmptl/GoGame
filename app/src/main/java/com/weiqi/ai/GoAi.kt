package com.weiqi.ai

import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent

interface GoAi {
    fun chooseMove(state: GameState): MoveIntent
}

enum class AiDifficulty(val label: String, val description: String) {
    BEGINNER("Beginner", "Reactive heuristics. Captures and saves stones in atari."),
    INTERMEDIATE("Intermediate", "Looks one move ahead. Avoids self-atari and weighs territory.")
}
