import 'package:flutter/material.dart';

import '../../data/drill_repo.dart';
import '../../domain/drills/drill.dart';
import '../components/zen_components.dart';

class DrillsScreen extends StatefulWidget {
  final DrillRepo repo;
  final ValueChanged<Drill> onOpen;

  const DrillsScreen({
    super.key,
    required this.repo,
    required this.onOpen,
  });

  @override
  State<DrillsScreen> createState() => _DrillsScreenState();
}

class _DrillsScreenState extends State<DrillsScreen> {
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
    final drills = widget.repo.drills;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Drills'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            Text(
              'Drills are short positions played against the AI. Beat the AI three times in a row to master each drill.',
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 12),
            for (final drill in drills) ...[
              _DrillCard(
                drill: drill,
                wins: widget.repo.winsFor(drill.id),
                mastered: widget.repo.isMastered(drill.id),
                onTap: () => widget.onOpen(drill),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
    );
  }
}

class _DrillCard extends StatelessWidget {
  final Drill drill;
  final int wins;
  final bool mastered;
  final VoidCallback onTap;

  const _DrillCard({
    required this.drill,
    required this.wins,
    required this.mastered,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final progress = (wins / drill.requiredWins).clamp(0, 1).toDouble();
    return ZenCard(
      onTap: onTap,
      container:
          mastered ? scheme.primaryContainer : scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                mastered ? Icons.workspace_premium : Icons.school,
                color: mastered ? scheme.onPrimaryContainer : scheme.primary,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(drill.title,
                    style: text.headlineSmall?.copyWith(
                      color: mastered
                          ? scheme.onPrimaryContainer
                          : scheme.onSurface,
                    )),
              ),
              ZenChip(
                text: drill.category.label,
                container: mastered
                    ? scheme.onPrimaryContainer.withValues(alpha: 0.15)
                    : scheme.surfaceContainerHigh,
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            drill.description,
            style: text.bodyMedium?.copyWith(
              color: mastered
                  ? scheme.onPrimaryContainer.withValues(alpha: 0.85)
                  : scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: LinearProgressIndicator(
                    value: progress,
                    minHeight: 6,
                    backgroundColor: scheme.surfaceContainerHigh,
                    valueColor: AlwaysStoppedAnimation(
                      mastered ? scheme.onPrimaryContainer : scheme.primary,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Text(
                mastered ? 'Mastered' : '$wins / ${drill.requiredWins}',
                style: text.labelMedium?.copyWith(
                  color: mastered
                      ? scheme.onPrimaryContainer
                      : scheme.onSurface,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
