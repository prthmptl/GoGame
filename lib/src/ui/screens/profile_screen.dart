import 'package:flutter/material.dart';

import '../../data/drill_repo.dart';
import '../../data/lesson_repo.dart';
import '../../data/profile_store.dart';
import '../../data/puzzle_repo.dart';
import '../../data/saved_game_repo.dart';
import '../components/zen_components.dart';

class ProfileScreen extends StatefulWidget {
  final ProfileStore profile;
  final SavedGameRepo games;
  final PuzzleRepo puzzles;
  final DrillRepo drills;
  final LessonRepo lessons;

  const ProfileScreen({
    super.key,
    required this.profile,
    required this.games,
    required this.puzzles,
    required this.drills,
    required this.lessons,
  });

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late Future<_Stats> _statsFuture;

  @override
  void initState() {
    super.initState();
    widget.profile.addListener(_onChanged);
    widget.puzzles.addListener(_onChanged);
    widget.drills.addListener(_onChanged);
    widget.lessons.addListener(_onChanged);
    _statsFuture = _gather();
  }

  @override
  void dispose() {
    widget.profile.removeListener(_onChanged);
    widget.puzzles.removeListener(_onChanged);
    widget.drills.removeListener(_onChanged);
    widget.lessons.removeListener(_onChanged);
    super.dispose();
  }

  void _onChanged() {
    if (!mounted) return;
    setState(() => _statsFuture = _gather());
  }

  Future<_Stats> _gather() async {
    final all = await widget.games.listAll();
    final completed =
        all.where((g) => g.status == 'COMPLETED' || g.status == 'RESIGNED');
    var played = 0;
    var wins = 0;
    var losses = 0;
    var draws = 0;
    final perSize = <int, int>{};
    for (final g in completed) {
      played++;
      perSize[g.boardSize] = (perSize[g.boardSize] ?? 0) + 1;
      final you = g.youColor.toUpperCase();
      final result = g.resultLabel;
      final didYouWin = _didPlayerWin(you, result);
      if (didYouWin == null) {
        draws++;
      } else if (didYouWin) {
        wins++;
      } else {
        losses++;
      }
    }
    final favSize = perSize.entries
        .fold<MapEntry<int, int>?>(null,
            (best, e) => best == null || e.value > best.value ? e : best)
        ?.key;
    final puzzlesSolved = widget.puzzles.attempts.values
        .where((a) => a.status == AttemptStatus.solved)
        .length;
    final drillsMastered =
        widget.drills.drills.where((d) => widget.drills.isMastered(d.id)).length;
    final lessonsCompleted = widget.lessons.courses
        .fold<int>(0, (acc, c) => acc + widget.lessons.completedCount(c));
    final totalLessons =
        widget.lessons.courses.fold<int>(0, (acc, c) => acc + c.lessons.length);
    return _Stats(
      played: played,
      wins: wins,
      losses: losses,
      draws: draws,
      favSize: favSize,
      puzzlesSolved: puzzlesSolved,
      puzzleTotal: widget.puzzles.puzzles.length,
      drillsMastered: drillsMastered,
      drillTotal: widget.drills.drills.length,
      lessonsCompleted: lessonsCompleted,
      lessonsTotal: totalLessons,
    );
  }

  /// Returns null on draw or unknown, true if `you` won, false if `you` lost.
  bool? _didPlayerWin(String youColor, String result) {
    if (result.isEmpty) return null;
    if (result.toLowerCase() == 'draw') return null;
    final winner = result.startsWith('B') ? 'BLACK' : 'WHITE';
    return winner == youColor;
  }

  Future<void> _editName() async {
    final controller = TextEditingController(text: widget.profile.value.name);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Display name'),
        content: TextField(controller: controller, autofocus: true),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Cancel')),
          TextButton(
              onPressed: () => Navigator.pop(ctx, controller.text),
              child: const Text('Save')),
        ],
      ),
    );
    if (result != null && result.trim().isNotEmpty) {
      await widget.profile
          .update((p) => p.copyWith(name: result.trim()));
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Profile'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: FutureBuilder<_Stats>(
          future: _statsFuture,
          builder: (ctx, snap) {
            final p = widget.profile.value;
            final s = snap.data;
            return ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                ZenCard(
                  container: scheme.primaryContainer,
                  child: Row(
                    children: [
                      Container(
                        width: 56,
                        height: 56,
                        decoration: BoxDecoration(
                          color: scheme.onPrimaryContainer
                              .withValues(alpha: 0.15),
                          shape: BoxShape.circle,
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          p.name.isEmpty ? '?' : p.name[0].toUpperCase(),
                          style: text.displayLarge?.copyWith(
                            fontSize: 28,
                            color: scheme.onPrimaryContainer,
                          ),
                        ),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(p.name,
                                style: text.headlineMedium?.copyWith(
                                    color: scheme.onPrimaryContainer)),
                            Text('Rating · ${p.rating}',
                                style: text.bodyMedium?.copyWith(
                                    color: scheme.onPrimaryContainer
                                        .withValues(alpha: 0.85))),
                          ],
                        ),
                      ),
                      IconButton(
                        onPressed: _editName,
                        icon: Icon(Icons.edit,
                            color: scheme.onPrimaryContainer),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                if (s == null)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 24),
                    child: Center(child: CircularProgressIndicator()),
                  )
                else ...[
                  _StatGrid(stats: s),
                  const SizedBox(height: 12),
                  _ProgressBlock(
                    title: 'Puzzles',
                    completed: s.puzzlesSolved,
                    total: s.puzzleTotal,
                    extraLabel: 'Daily puzzle streak is tracked in puzzles.',
                  ),
                  const SizedBox(height: 12),
                  _ProgressBlock(
                    title: 'Drills',
                    completed: s.drillsMastered,
                    total: s.drillTotal,
                    extraLabel: 'Each mastery = 3 wins in a row.',
                  ),
                  const SizedBox(height: 12),
                  _ProgressBlock(
                    title: 'Lessons',
                    completed: s.lessonsCompleted,
                    total: s.lessonsTotal,
                    extraLabel: 'Across all courses.',
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }
}

class _Stats {
  final int played;
  final int wins;
  final int losses;
  final int draws;
  final int? favSize;
  final int puzzlesSolved;
  final int puzzleTotal;
  final int drillsMastered;
  final int drillTotal;
  final int lessonsCompleted;
  final int lessonsTotal;
  const _Stats({
    required this.played,
    required this.wins,
    required this.losses,
    required this.draws,
    required this.favSize,
    required this.puzzlesSolved,
    required this.puzzleTotal,
    required this.drillsMastered,
    required this.drillTotal,
    required this.lessonsCompleted,
    required this.lessonsTotal,
  });
}

class _StatGrid extends StatelessWidget {
  final _Stats stats;
  const _StatGrid({required this.stats});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final winRate = stats.played == 0
        ? '—'
        : '${(100 * stats.wins / stats.played).round()}%';
    final fav = stats.favSize == null ? '—' : '${stats.favSize}×${stats.favSize}';
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Game record', style: text.headlineSmall),
          const SizedBox(height: 12),
          Row(
            children: [
              _Stat(label: 'Played', value: '${stats.played}'),
              _Stat(label: 'Wins', value: '${stats.wins}'),
              _Stat(label: 'Losses', value: '${stats.losses}'),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              _Stat(label: 'Win rate', value: winRate),
              _Stat(label: 'Draws', value: '${stats.draws}'),
              _Stat(label: 'Fav size', value: fav),
            ],
          ),
        ],
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
    return Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label.toUpperCase(),
              style: text.labelSmall?.copyWith(
                  color: scheme.onSurfaceVariant, letterSpacing: 1.2)),
          Text(value, style: text.headlineSmall),
        ],
      ),
    );
  }
}

class _ProgressBlock extends StatelessWidget {
  final String title;
  final int completed;
  final int total;
  final String extraLabel;
  const _ProgressBlock({
    required this.title,
    required this.completed,
    required this.total,
    required this.extraLabel,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final pct = total == 0 ? 0.0 : completed / total;
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(title, style: text.headlineSmall)),
              Text('$completed / $total',
                  style: text.labelMedium
                      ?.copyWith(color: scheme.onSurfaceVariant)),
            ],
          ),
          const SizedBox(height: 8),
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: LinearProgressIndicator(
              value: pct,
              minHeight: 6,
              backgroundColor: scheme.surfaceContainerHigh,
              valueColor: AlwaysStoppedAnimation(scheme.primary),
            ),
          ),
          const SizedBox(height: 6),
          Text(extraLabel,
              style: text.labelSmall
                  ?.copyWith(color: scheme.onSurfaceVariant)),
        ],
      ),
    );
  }
}
