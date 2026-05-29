import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../data/puzzle_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/models.dart';
import '../../domain/puzzles/puzzle.dart';
import '../../domain/puzzles/puzzle_session.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

class PuzzleScreen extends StatefulWidget {
  final Puzzle puzzle;
  final PuzzleRepo repo;
  final SettingsStore settings;
  final bool isDaily;

  const PuzzleScreen({
    super.key,
    required this.puzzle,
    required this.repo,
    required this.settings,
    this.isDaily = false,
  });

  @override
  State<PuzzleScreen> createState() => _PuzzleScreenState();
}

class _PuzzleScreenState extends State<PuzzleScreen> {
  late PuzzleSession _session;
  Point? _hintPoint;
  bool _recorded = false;

  @override
  void initState() {
    super.initState();
    _session = PuzzleSession.start(widget.puzzle);
  }

  void _onTap(Point point) {
    if (_session.status != PuzzleStatus.inProgress) return;
    final ok = _session.play(point);
    if (ok) {
      HapticFeedback.lightImpact();
    } else {
      HapticFeedback.heavyImpact();
    }
    setState(() => _hintPoint = null);
    if (_session.status != PuzzleStatus.inProgress && !_recorded) {
      _recordAttempt();
    }
  }

  Future<void> _recordAttempt() async {
    _recorded = true;
    final res = _session.finalize();
    final attempt = PuzzleAttempt(
      puzzleId: widget.puzzle.id,
      status: res.status == PuzzleStatus.solved
          ? AttemptStatus.solved
          : AttemptStatus.failed,
      mistakes: res.mistakes,
      hintsUsed: res.hintsUsed,
      elapsedMillis: res.elapsed.inMilliseconds,
      completedAtMillis: DateTime.now().millisecondsSinceEpoch,
    );
    await widget.repo.recordAttempt(attempt);
    if (widget.isDaily && res.status == PuzzleStatus.solved) {
      await widget.repo.markDailySolved();
    }
  }

  void _retry() {
    setState(() {
      _session.retry();
      _hintPoint = null;
      _recorded = false;
    });
  }

  void _hint() {
    final point = _session.hint();
    if (point == null) return;
    HapticFeedback.selectionClick();
    setState(() => _hintPoint = point);
  }

  BoardAppearance _appearance(BoardThemeKind kind) => switch (kind) {
        BoardThemeKind.classicWood => BoardAppearance.classicWood,
        BoardThemeKind.minimalPaper => BoardAppearance.minimalPaper,
        BoardThemeKind.darkSlate => BoardAppearance.darkSlate,
        BoardThemeKind.highContrast => BoardAppearance.highContrast,
      }
          .copyWith(showCoordinates: widget.settings.value.showCoordinates);

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final state = _session.state;
    final overlay = BoardOverlay(
      lastMove: state.lastMove?.point,
      markers: _hintPoint == null ? const {} : {_hintPoint!},
    );

    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
        title: Text(widget.puzzle.title),
        actions: [
          if (widget.isDaily)
            const Padding(
              padding: EdgeInsets.only(right: 12),
              child: Center(
                  child: ZenChip(
                      text: 'DAILY',
                      container: Color(0xFFE8DFD0),
                      contentColor: Color(0xFF361F1A))),
            ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          children: [
            ZenCard(
              container: scheme.surfaceContainerLow,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          '${widget.puzzle.theme.label} · ${_toMoveLabel(_session.toMove)} to play',
                          style: text.labelMedium
                              ?.copyWith(color: scheme.onSurfaceVariant),
                        ),
                      ),
                      ZenChip(
                        text: 'Level ${widget.puzzle.difficulty}',
                        container: scheme.surfaceContainerHigh,
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(widget.puzzle.description, style: text.bodyLarge),
                ],
              ),
            ),
            const SizedBox(height: 12),
            AspectRatio(
              aspectRatio: 1,
              child: ZenCard(
                contentPadding: const EdgeInsets.all(6),
                child: BoardCanvas(
                  board: state.board,
                  overlay: overlay,
                  appearance: _appearance(widget.settings.value.boardTheme),
                  onTap: _onTap,
                ),
              ),
            ),
            const SizedBox(height: 12),
            _StatusBanner(
              status: _session.status,
              mistakes: _session.mistakes,
              hintsUsed: _session.hintsUsed,
              lastComment: _session.lastComment,
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _session.status == PuzzleStatus.inProgress
                        ? _hint
                        : null,
                    icon: const Icon(Icons.lightbulb_outline),
                    label: const Text('Hint'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton.icon(
                    onPressed: _retry,
                    icon: const Icon(Icons.refresh),
                    label: const Text('Retry'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  static String _toMoveLabel(StoneColor c) =>
      c == StoneColor.black ? 'Black' : 'White';
}

class _StatusBanner extends StatelessWidget {
  final PuzzleStatus status;
  final int mistakes;
  final int hintsUsed;
  final String? lastComment;

  const _StatusBanner({
    required this.status,
    required this.mistakes,
    required this.hintsUsed,
    required this.lastComment,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final (bg, fg, title) = switch (status) {
      PuzzleStatus.inProgress => (
        scheme.surfaceContainerHigh,
        scheme.onSurface,
        mistakes == 0 ? 'Your move' : 'Try again',
      ),
      PuzzleStatus.solved => (
        scheme.primary.withValues(alpha: 0.12),
        scheme.primary,
        'Solved!',
      ),
      PuzzleStatus.failed => (
        scheme.error.withValues(alpha: 0.12),
        scheme.error,
        'Wrong line',
      ),
    };
    final subtitleParts = <String>[];
    if (lastComment != null && lastComment!.isNotEmpty) {
      subtitleParts.add(lastComment!);
    }
    if (mistakes > 0) {
      subtitleParts.add('Mistakes: $mistakes');
    }
    if (hintsUsed > 0) {
      subtitleParts.add('Hints: $hintsUsed');
    }
    return ZenCard(
      container: bg,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: text.headlineSmall?.copyWith(color: fg)),
          if (subtitleParts.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(subtitleParts.join(' · '),
                style: text.bodyMedium?.copyWith(color: fg)),
          ],
        ],
      ),
    );
  }
}
