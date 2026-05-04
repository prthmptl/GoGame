package com.weiqi.engine

/** Liberty count of the group containing [start]. Returns 0 if [start] is empty. */
internal fun internalLiberties(board: Board, start: Point): Int {
    if (board.get(start) == CellState.EMPTY) return 0
    return Groups.findGroup(board, start).liberties.size
}
