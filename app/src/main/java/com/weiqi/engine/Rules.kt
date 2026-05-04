package com.weiqi.engine

object Rules {

    fun apply(state: GameState, intent: MoveIntent): MoveResult {
        if (state.status != GameStatus.ACTIVE) {
            return MoveResult.Rejected(MoveResult.Reason.GAME_NOT_ACTIVE)
        }
        return when (intent.type) {
            MoveType.PASS -> applyPass(state)
            MoveType.RESIGN -> applyResign(state)
            MoveType.PLACE_STONE -> applyPlace(state, intent.point!!)
        }
    }

    private fun applyPass(state: GameState): MoveResult {
        val passes = state.consecutivePasses + 1
        val move = Move(state.moveNumber + 1, state.currentPlayer, MoveType.PASS, null, emptyList())
        val nextStatus = if (passes >= 2) GameStatus.SCORING else GameStatus.ACTIVE
        return MoveResult.Accepted(
            state.copy(
                currentPlayer = state.currentPlayer.other(),
                moveNumber = state.moveNumber + 1,
                koPoint = null,
                consecutivePasses = passes,
                status = nextStatus,
                lastMove = move,
                history = state.history + move
            ),
            move
        )
    }

    private fun applyResign(state: GameState): MoveResult {
        val move = Move(state.moveNumber + 1, state.currentPlayer, MoveType.RESIGN, null, emptyList())
        return MoveResult.Accepted(
            state.copy(
                status = GameStatus.RESIGNED,
                lastMove = move,
                history = state.history + move
            ),
            move
        )
    }

    private fun applyPlace(state: GameState, point: Point): MoveResult {
        val board = state.board
        if (!board.inBounds(point)) return MoveResult.Rejected(MoveResult.Reason.OUT_OF_BOUNDS)
        if (board.get(point) != CellState.EMPTY) return MoveResult.Rejected(MoveResult.Reason.OCCUPIED)
        if (state.koPoint == point) return MoveResult.Rejected(MoveResult.Reason.KO_VIOLATION)

        val player = state.currentPlayer
        val opponent = player.other()
        val placed = board.set(point, CellState.of(player))

        // Capture opponent groups with no liberties touching the placed stone.
        val captured = HashSet<Point>()
        val seen = HashSet<Point>()
        for (n in placed.neighbors(point)) {
            if (placed.get(n) == CellState.of(opponent) && n !in seen) {
                val group = Groups.findGroup(placed, n)
                seen.addAll(group.stones)
                if (group.liberties.isEmpty()) captured.addAll(group.stones)
            }
        }
        val afterCapture = if (captured.isEmpty()) placed
        else placed.setMany(captured.map { it to CellState.EMPTY })

        // Suicide check: own group must have liberties.
        val ownGroup = Groups.findGroup(afterCapture, point)
        if (ownGroup.liberties.isEmpty() && !state.config.allowSuicide) {
            return MoveResult.Rejected(MoveResult.Reason.SUICIDE)
        }

        // Superko: position must not repeat.
        val newHash = afterCapture.zobristHash()
        if (state.config.useSuperko && newHash in state.previousHashes) {
            return MoveResult.Rejected(MoveResult.Reason.SUPERKO_VIOLATION)
        }

        // Simple ko marker: exactly one stone captured and the placed stone is alone with one liberty.
        val koPoint: Point? =
            if (captured.size == 1 && ownGroup.stones.size == 1 && ownGroup.liberties.size == 1) {
                captured.first()
            } else null

        val move = Move(
            moveNumber = state.moveNumber + 1,
            player = player,
            type = MoveType.PLACE_STONE,
            point = point,
            captured = captured.toList()
        )

        val capsByBlack = state.capturesByBlack + if (player == StoneColor.BLACK) captured.size else 0
        val capsByWhite = state.capturesByWhite + if (player == StoneColor.WHITE) captured.size else 0

        return MoveResult.Accepted(
            state.copy(
                board = afterCapture,
                currentPlayer = opponent,
                moveNumber = state.moveNumber + 1,
                capturesByBlack = capsByBlack,
                capturesByWhite = capsByWhite,
                koPoint = koPoint,
                previousHashes = state.previousHashes + newHash,
                consecutivePasses = 0,
                lastMove = move,
                history = state.history + move
            ),
            move
        )
    }

    /** Returns the list of legal placement points for the given player. Pass/resign are always legal. */
    fun legalPlacements(state: GameState): List<Point> {
        if (state.status != GameStatus.ACTIVE) return emptyList()
        val out = ArrayList<Point>()
        val s = state.board.size
        for (r in 0 until s) for (c in 0 until s) {
            val p = Point(r, c)
            if (state.board.get(p) != CellState.EMPTY) continue
            val res = apply(state, MoveIntent(MoveType.PLACE_STONE, p))
            if (res is MoveResult.Accepted) out.add(p)
        }
        return out
    }
}
