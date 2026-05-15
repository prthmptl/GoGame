import 'package:flutter/material.dart';

import '../../domain/models.dart';
import '../board/mini_stone.dart';
import '../components/zen_components.dart';

class RecentGame {
  final String id;
  final String opponent;
  final String result;
  final int boardSize;
  final String date;
  final StoneColor youPlayed;
  final String? sgfPath;

  const RecentGame({
    required this.id,
    required this.opponent,
    required this.result,
    required this.boardSize,
    required this.date,
    required this.youPlayed,
    required this.sgfPath,
  });
}

class HomeScreen extends StatelessWidget {
  final VoidCallback onPlayLocal;
  final VoidCallback onPlayAi;
  final VoidCallback onRules;
  final VoidCallback? onResume;
  final List<RecentGame> recents;
  final ValueChanged<RecentGame> onOpenRecent;
  final ValueChanged<RecentGame> onShareRecent;
  final ValueChanged<RecentGame> onDeleteRecent;

  const HomeScreen({
    super.key,
    required this.onPlayLocal,
    required this.onPlayAi,
    required this.onRules,
    this.onResume,
    this.recents = const [],
    required this.onOpenRecent,
    required this.onShareRecent,
    required this.onDeleteRecent,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Hero card
          ZenCard(
            container: scheme.surfaceContainerLow,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.center,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  'Find Your Focus',
                  style: text.displayLarge?.copyWith(height: 48 / 42),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 8),
                Text(
                  'Step into the quiet space.',
                  style: text.bodyMedium
                      ?.copyWith(color: scheme.onSurfaceVariant),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                SizedBox(
                  width: double.infinity,
                  height: 56,
                  child: FilledButton.icon(
                    onPressed: onPlayLocal,
                    icon: const Icon(Icons.play_arrow),
                    label: Text('Play Now',
                        style: text.labelLarge
                            ?.copyWith(color: scheme.onPrimary)),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          ZenCard(
            container: scheme.surfaceContainerHigh,
            onTap: onRules,
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Rules', style: text.headlineSmall),
                      Text('Complete Chinese rules reference.',
                          style: text.bodyMedium
                              ?.copyWith(color: scheme.onSurfaceVariant)),
                    ],
                  ),
                ),
                Icon(Icons.menu_book, color: scheme.primary),
              ],
            ),
          ),
          if (onResume != null) ...[
            const SizedBox(height: 12),
            ZenCard(
              container: scheme.primaryContainer,
              onTap: onResume,
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('Continue Game',
                            style: text.headlineSmall
                                ?.copyWith(color: scheme.onPrimaryContainer)),
                        Text('Pick up where you left off.',
                            style: text.bodyMedium?.copyWith(
                                color: scheme.onPrimaryContainer
                                    .withValues(alpha: 0.85))),
                      ],
                    ),
                  ),
                  const MiniStone(color: StoneColor.white, size: 36),
                ],
              ),
            ),
          ],
          const SizedBox(height: 12),
          ZenCard(
            container: scheme.surfaceContainerHigh,
            onTap: onPlayAi,
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Play vs AI', style: text.headlineSmall),
                      Text('Beginner engine.',
                          style: text.bodyMedium
                              ?.copyWith(color: scheme.onSurfaceVariant)),
                    ],
                  ),
                ),
                const MiniStone(color: StoneColor.black, size: 36),
              ],
            ),
          ),
          if (recents.isNotEmpty) ...[
            const SizedBox(height: 12),
            ZenCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Recent Games', style: text.headlineSmall),
                      Text('LATEST ${recents.length}',
                          style:
                              text.labelSmall?.copyWith(color: scheme.primary)),
                    ],
                  ),
                  const SizedBox(height: 8),
                  ...recents.map((game) => _RecentGameRow(
                        game: game,
                        onOpen: () => onOpenRecent(game),
                        onShare: () => onShareRecent(game),
                        onDelete: () => onDeleteRecent(game),
                      )),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _RecentGameRow extends StatelessWidget {
  final RecentGame game;
  final VoidCallback onOpen;
  final VoidCallback onShare;
  final VoidCallback onDelete;

  const _RecentGameRow({
    required this.game,
    required this.onOpen,
    required this.onShare,
    required this.onDelete,
  });

  Future<void> _confirmDelete(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete recent game?'),
        content: const Text(
            'This removes the saved game and its SGF file from your history.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancel')),
          TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Delete')),
        ],
      ),
    );
    if (confirmed == true) onDelete();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return InkWell(
      onTap: game.sgfPath != null ? onOpen : null,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: scheme.surfaceContainerHigh,
              ),
              alignment: Alignment.center,
              child: MiniStone(color: game.youPlayed),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('vs. ${game.opponent}',
                      style: text.bodyMedium
                          ?.copyWith(fontWeight: FontWeight.w600)),
                  Text(
                    '${game.result} · ${game.boardSize}×${game.boardSize} · ${game.date}',
                    style: text.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                ],
              ),
            ),
            if (game.sgfPath != null)
              IconButton(
                onPressed: onShare,
                icon: const Icon(Icons.ios_share),
                color: scheme.primary,
                tooltip: 'Share SGF',
              ),
            IconButton(
              onPressed: () => _confirmDelete(context),
              icon: const Icon(Icons.delete),
              color: scheme.onSurfaceVariant,
              tooltip: 'Delete recent game',
            ),
          ],
        ),
      ),
    );
  }
}
