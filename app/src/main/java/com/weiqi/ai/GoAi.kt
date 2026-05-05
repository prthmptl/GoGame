package com.weiqi.ai

import com.weiqi.engine.GameState
import com.weiqi.engine.MoveIntent

interface GoAi {
    fun chooseMove(state: GameState): MoveIntent
}
