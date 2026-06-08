import '../board.dart';
import '../game_state.dart';
import '../models.dart';
import '../rules.dart';
import 'puzzle.dart';

enum PuzzleStatus { inProgress, solved, failed }

class PuzzleResult {
  final PuzzleStatus status;
  final int mistakes;
  final int hintsUsed;
  final Duration elapsed;

  const PuzzleResult({
    required this.status,
    required this.mistakes,
    required this.hintsUsed,
    required this.elapsed,
  });
}

/// Stateful, mutable controller for working through a single puzzle.
///
/// The session maintains a real [GameState] for rendering and capture handling,
/// but uses the puzzle's solution tree as the source of truth for what counts
/// as a correct move.
class PuzzleSession {
  final Puzzle puzzle;
  final DateTime _startedAt;

  GameState _state;
  List<PuzzleNode> _expected;
  PuzzleStatus _status = PuzzleStatus.inProgress;
  int _mistakes = 0;
  int _hintsUsed = 0;
  String? _lastComment;
  Point? _lastWrongMove;

  PuzzleSession._(this.puzzle, this._state, this._expected)
      : _startedAt = DateTime.now();

  factory PuzzleSession.start(Puzzle puzzle) {
    final state = _seedState(puzzle);
    return PuzzleSession._(puzzle, state, puzzle.solution);
  }

  GameState get state => _state;
  PuzzleStatus get status => _status;
  int get mistakes => _mistakes;
  int get hintsUsed => _hintsUsed;
  String? get lastComment => _lastComment;
  Point? get lastWrongMove => _lastWrongMove;
  StoneColor get toMove => _state.currentPlayer;

  /// Submit a move at [point]. Returns whether the move was on the solution
  /// path. Off-path moves count as a mistake; the session stays in progress
  /// so the user can retry.
  bool play(Point point) {
    if (_status != PuzzleStatus.inProgress) return false;
    final match = _findChild(point, _state.currentPlayer);
    if (match == null) {
      _mistakes += 1;
      _lastWrongMove = point;
      _lastComment = 'Not the right move — try again.';
      return false;
    }
    _lastWrongMove = null;
    _lastComment = match.comment;
    _advance(match);
    return true;
  }

  /// Reveal the next expected move (does not advance the state).
  Point? hint() {
    if (_status != PuzzleStatus.inProgress) return null;
    if (_expected.isEmpty) return null;
    _hintsUsed += 1;
    return _expected.first.move.point;
  }

  /// Reset the session to its initial state (keeps mistake/hint counters).
  void retry() {
    _state = _seedState(puzzle);
    _expected = puzzle.solution;
    _status = PuzzleStatus.inProgress;
    _lastComment = null;
    _lastWrongMove = null;
  }

  PuzzleResult finalize() => PuzzleResult(
        status: _status,
        mistakes: _mistakes,
        hintsUsed: _hintsUsed,
        elapsed: DateTime.now().difference(_startedAt),
      );

  PuzzleNode? _findChild(Point point, StoneColor player) {
    for (final node in _expected) {
      if (node.move.point == point && node.move.player == player) {
        return node;
      }
    }
    return null;
  }

  void _advance(PuzzleNode node) {
    final applied = _applyMove(node.move);
    if (!applied && node.outcome != NodeOutcome.wrong) {
      _mistakes += 1;
      _lastWrongMove = node.move.point;
      _lastComment = 'That puzzle line is not legal from this position.';
      return;
    }
    if (node.outcome == NodeOutcome.correct) {
      _status = PuzzleStatus.solved;
      _expected = const [];
      return;
    }
    if (node.outcome == NodeOutcome.wrong) {
      _status = PuzzleStatus.failed;
      _expected = const [];
      return;
    }
    final replies = node.children;
    if (replies.isEmpty) {
      // No further moves modelled; treat as solved unless the line was
      // explicitly marked wrong/correct.
      _status = PuzzleStatus.solved;
      _expected = const [];
      return;
    }
    // Opponent's forced reply: take the first child (puzzles are authored so
    // the first child of an opponent move is the canonical response).
    final reply = replies.first;
    if (reply.move.player != _state.currentPlayer) {
      // The next node is the user's turn again; just expose those children.
      _expected = replies;
      return;
    }
    final replyApplied = _applyMove(reply.move);
    if (!replyApplied) {
      _status = PuzzleStatus.failed;
      _expected = const [];
      _lastComment = 'The puzzle response is not legal from this position.';
      return;
    }
    if (reply.outcome == NodeOutcome.correct) {
      _status = PuzzleStatus.solved;
      _expected = const [];
      return;
    }
    if (reply.outcome == NodeOutcome.wrong) {
      _status = PuzzleStatus.failed;
      _expected = const [];
      return;
    }
    if (reply.children.isEmpty) {
      _status = PuzzleStatus.solved;
      _expected = const [];
      return;
    }
    _expected = reply.children;
  }

  bool _applyMove(PuzzleMove move) {
    final res = Rules.apply(_state, MoveIntent.place(move.point));
    if (res.isAccepted) {
      _state = res.newStateAs<GameState>();
      return true;
    }
    return false;
  }

  static GameState _seedState(Puzzle puzzle) {
    var board = Board.empty(puzzle.boardSize);
    board = board.setMany([
      for (final p in puzzle.initialBlack) MapEntry(p, CellState.black),
      for (final p in puzzle.initialWhite) MapEntry(p, CellState.white),
    ]);
    final config = GameConfig(boardSize: puzzle.boardSize);
    final seedHash =
        Rules.positionHash(config.superkoMode, board, puzzle.toMove);
    return GameState(
      board: board,
      config: config,
      currentPlayer: puzzle.toMove,
      moveNumber: 0,
      capturesByBlack: 0,
      capturesByWhite: 0,
      koPoint: null,
      previousHashes: <int>{seedHash},
      status: GameStatus.active,
      consecutivePasses: 0,
      lastMove: null,
      history: const [],
    );
  }
}
