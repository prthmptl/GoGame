import 'package:flutter/material.dart';

import '../../domain/bots/bot_catalog.dart';
import '../../domain/bots/bot_profile.dart';
import '../components/zen_components.dart';

class BotPickerScreen extends StatelessWidget {
  final BotProfile? selected;

  const BotPickerScreen({
    super.key,
    required this.selected,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final bots = BotCatalog.sortedByRating();
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Choose an opponent'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            Text(
              'Every opponent has their own play style. Higher rated bots punish mistakes; lower rated ones forgive them.',
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 12),
            for (final bot in bots) ...[
              _BotCard(
                bot: bot,
                selected: bot.id == selected?.id,
                onTap: () => Navigator.of(context).pop(bot),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
    );
  }
}

class _BotCard extends StatelessWidget {
  final BotProfile bot;
  final bool selected;
  final VoidCallback onTap;

  const _BotCard({
    required this.bot,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return ZenCard(
      container:
          selected ? scheme.primaryContainer : scheme.surfaceContainerLow,
      onTap: onTap,
      child: Row(
        children: [
          _Avatar(initials: bot.initials, selected: selected),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(bot.name,
                        style: text.headlineSmall?.copyWith(
                          color: selected
                              ? scheme.onPrimaryContainer
                              : scheme.onSurface,
                        )),
                    const SizedBox(width: 8),
                    Text(bot.countryEmoji, style: const TextStyle(fontSize: 18)),
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  '${bot.rankLabel} · ${bot.style.label}',
                  style: text.labelSmall?.copyWith(
                    color: selected
                        ? scheme.onPrimaryContainer.withValues(alpha: 0.85)
                        : scheme.onSurfaceVariant,
                    letterSpacing: 1.1,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  bot.bio,
                  style: text.bodyMedium?.copyWith(
                    color: selected
                        ? scheme.onPrimaryContainer
                        : scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Avatar extends StatelessWidget {
  final String initials;
  final bool selected;
  const _Avatar({required this.initials, required this.selected});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Container(
      width: 48,
      height: 48,
      decoration: BoxDecoration(
        color: selected
            ? scheme.onPrimaryContainer.withValues(alpha: 0.15)
            : scheme.surfaceContainerHigh,
        shape: BoxShape.circle,
      ),
      alignment: Alignment.center,
      child: Text(
        initials,
        style: text.headlineSmall?.copyWith(
          fontWeight: FontWeight.w700,
          letterSpacing: 1,
          color: selected ? scheme.onPrimaryContainer : scheme.onSurface,
        ),
      ),
    );
  }
}
