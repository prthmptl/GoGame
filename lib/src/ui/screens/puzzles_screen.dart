import 'package:flutter/material.dart';

import '../../data/puzzle_repo.dart';
import '../../domain/puzzles/puzzle.dart';
import '../components/zen_components.dart';

class PuzzlesScreen extends StatefulWidget {
  final PuzzleRepo repo;
  final ValueChanged<Puzzle> onOpenPuzzle;
  final VoidCallback onStartRush;

  const PuzzlesScreen({
    super.key,
    required this.repo,
    required this.onOpenPuzzle,
    required this.onStartRush,
  });

  @override
  State<PuzzlesScreen> createState() => _PuzzlesScreenState();
}

class _PuzzlesScreenState extends State<PuzzlesScreen> {
  PuzzleTheme? _filter;

  @override
  void initState() {
    super.initState();
    widget.repo.addListener(_onChanged);
  }

  @override
  void dispose() {
    widget.repo.removeListener(_onChanged);
    super.dispose();
  }

  void _onChanged() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final repo = widget.repo;
    final all = repo.puzzles;
    final filtered =
        _filter == null ? all : all.where((p) => p.theme == _filter).toList();
    final daily = repo.dailyPuzzle();
    final streak = repo.streak();
    final solvedCount = repo.attempts.values
        .where((a) => a.status == AttemptStatus.solved)
        .length;

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ZenCard(
            container: scheme.surfaceContainerLow,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Puzzles', style: text.displayLarge?.copyWith(height: 1.1)),
                const SizedBox(height: 4),
                Text(
                  'Sharpen your reading. One move at a time.',
                  style: text.bodyMedium
                      ?.copyWith(color: scheme.onSurfaceVariant),
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    _Stat(label: 'Solved', value: '$solvedCount / ${all.length}'),
                    const SizedBox(width: 16),
                    _Stat(label: 'Streak', value: '${streak.current}d'),
                    const SizedBox(width: 16),
                    _Stat(label: 'Best', value: '${streak.best}d'),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          ZenCard(
            container: scheme.surfaceContainerHigh,
            onTap: widget.onStartRush,
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('PUZZLE RUSH',
                          style: text.labelSmall
                              ?.copyWith(letterSpacing: 1.4)),
                      const SizedBox(height: 4),
                      Text('3-minute sprint',
                          style: text.headlineSmall),
                      const SizedBox(height: 2),
                      Text(
                        'Solve as many as you can. Best: ${repo.rushBest()}',
                        style: text.bodyMedium?.copyWith(
                            color: scheme.onSurfaceVariant),
                      ),
                    ],
                  ),
                ),
                Icon(Icons.timer, color: scheme.primary, size: 28),
              ],
            ),
          ),
          const SizedBox(height: 12),
          if (daily != null) ...[
            ZenCard(
              container: scheme.primaryContainer,
              onTap: () => widget.onOpenPuzzle(daily),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('DAILY PUZZLE',
                            style: text.labelSmall?.copyWith(
                                color: scheme.onPrimaryContainer,
                                letterSpacing: 1.4)),
                        const SizedBox(height: 4),
                        Text(daily.title,
                            style: text.headlineSmall
                                ?.copyWith(color: scheme.onPrimaryContainer)),
                        const SizedBox(height: 2),
                        Text(daily.description,
                            style: text.bodyMedium?.copyWith(
                                color: scheme.onPrimaryContainer
                                    .withValues(alpha: 0.85))),
                      ],
                    ),
                  ),
                  Icon(Icons.play_arrow, color: scheme.onPrimaryContainer),
                ],
              ),
            ),
            const SizedBox(height: 12),
          ],
          ZenCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('Themes', style: text.headlineSmall),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    _filterChip(context, 'All', _filter == null,
                        () => setState(() => _filter = null)),
                    for (final t in PuzzleTheme.values)
                      _filterChip(
                          context,
                          t.label,
                          _filter == t,
                          () => setState(() => _filter = t)),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          if (filtered.isEmpty)
            ZenCard(
              child: Text(
                'No puzzles in this theme yet.',
                style:
                    text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
              ),
            )
          else
            ZenCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  for (var i = 0; i < filtered.length; i++) ...[
                    _PuzzleRow(
                      puzzle: filtered[i],
                      attempt: repo.attemptFor(filtered[i].id),
                      onTap: () => widget.onOpenPuzzle(filtered[i]),
                    ),
                    if (i < filtered.length - 1)
                      Divider(
                          height: 1,
                          color: scheme.outlineVariant.withValues(alpha: 0.5)),
                  ],
                ],
              ),
            ),
        ],
      ),
    );
  }

  Widget _filterChip(
      BuildContext context, String label, bool selected, VoidCallback onTap) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: selected ? scheme.primary : scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(999),
        ),
        child: Text(
          label,
          style: text.labelMedium?.copyWith(
            color: selected ? scheme.onPrimary : scheme.onSurface,
          ),
        ),
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  final String label;
  final String value;
  const _Stat({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label.toUpperCase(),
            style: text.labelSmall?.copyWith(
                color: scheme.onSurfaceVariant, letterSpacing: 1.2)),
        Text(value, style: text.headlineSmall),
      ],
    );
  }
}

class _PuzzleRow extends StatelessWidget {
  final Puzzle puzzle;
  final PuzzleAttempt? attempt;
  final VoidCallback onTap;

  const _PuzzleRow({
    required this.puzzle,
    required this.attempt,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final solved = attempt?.status == AttemptStatus.solved;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: solved
                    ? scheme.primary.withValues(alpha: 0.12)
                    : scheme.surfaceContainerHigh,
              ),
              alignment: Alignment.center,
              child: Icon(
                solved ? Icons.check : Icons.extension,
                size: 20,
                color: solved ? scheme.primary : scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(puzzle.title,
                      style: text.bodyMedium
                          ?.copyWith(fontWeight: FontWeight.w600)),
                  Text(
                    '${puzzle.theme.label} · Level ${puzzle.difficulty}',
                    style: text.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                ],
              ),
            ),
            if (attempt != null)
              Padding(
                padding: const EdgeInsets.only(right: 8),
                child: ZenChip(
                  text: solved ? 'Solved' : 'Failed',
                  container: solved
                      ? scheme.primary.withValues(alpha: 0.12)
                      : scheme.error.withValues(alpha: 0.12),
                  contentColor: solved ? scheme.primary : scheme.error,
                ),
              ),
            Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
          ],
        ),
      ),
    );
  }
}
