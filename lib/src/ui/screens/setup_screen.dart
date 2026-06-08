import 'package:flutter/material.dart';

import '../../domain/bots/bot_catalog.dart';
import '../../domain/bots/bot_profile.dart';
import '../../domain/clock/time_control.dart';
import '../../domain/models.dart';
import '../components/zen_components.dart';
import 'bot_picker_screen.dart';
import 'game_view_model.dart';

class GameSetup {
  final GameConfig config;
  final Opponent opponent;
  final StoneColor aiColor;
  final AiDifficulty aiDifficulty;
  final TimeControl timeControl;
  final BotProfile? bot;
  const GameSetup({
    required this.config,
    required this.opponent,
    required this.aiColor,
    required this.aiDifficulty,
    required this.timeControl,
    this.bot,
  });
}

enum _TimePreset { noClock, blitz, rapid, fischer5p3, byoYomi, canadian }

extension _TimePresetX on _TimePreset {
  String get label => switch (this) {
        _TimePreset.noClock => 'None',
        _TimePreset.blitz => '5 min',
        _TimePreset.rapid => '15 min',
        _TimePreset.fischer5p3 => '5 + 3',
        _TimePreset.byoYomi => 'Byo-yomi',
        _TimePreset.canadian => 'Canadian',
      };

  TimeControl get control => switch (this) {
        _TimePreset.noClock => const TimeControl.none(),
        _TimePreset.blitz => const TimeControl.absolute(mainSeconds: 5 * 60),
        _TimePreset.rapid => const TimeControl.absolute(mainSeconds: 15 * 60),
        _TimePreset.fischer5p3 =>
          const TimeControl.fischer(mainSeconds: 5 * 60, incrementSeconds: 3),
        _TimePreset.byoYomi => const TimeControl.byoYomi(
            mainSeconds: 10 * 60, periods: 3, periodSeconds: 30),
        _TimePreset.canadian => const TimeControl.canadian(
            mainSeconds: 10 * 60, stonesPerPeriod: 20, periodSeconds: 5 * 60),
      };
}

class SetupScreen extends StatefulWidget {
  final bool isAi;
  final ValueChanged<GameSetup> onStart;

  const SetupScreen({super.key, required this.isAi, required this.onStart});

  @override
  State<SetupScreen> createState() => _SetupScreenState();
}

class _SetupScreenState extends State<SetupScreen> {
  int size = 9;
  Ruleset ruleset = Ruleset.chinese;
  double komi = 7.5;
  int handicap = 0;
  StoneColor aiColor = StoneColor.white;
  AiDifficulty aiDifficulty = AiDifficulty.beginner;
  _TimePreset timePreset = _TimePreset.rapid;
  BotProfile? selectedBot = BotCatalog.byId('kiri');
  GameVariant variant = GameVariant.standard;

  void _onRulesetChanged(Ruleset r) {
    final defaults = RulesetDefaults.of(r);
    setState(() {
      ruleset = r;
      komi = defaults.komi;
    });
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final defaults = RulesetDefaults.of(ruleset);
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(widget.isAi ? 'Practice Match' : 'Local Match',
              style:
                  text.labelMedium?.copyWith(color: scheme.onSurfaceVariant)),
          Text('Game setup',
              style:
                  text.headlineMedium?.copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 12),
          ZenCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _RulesetSection(
                  selected: ruleset,
                  onSelect: _onRulesetChanged,
                ),
                const SizedBox(height: 20),
                _ChipSection<GameVariant>(
                  title: 'GAME MODE',
                  options: GameVariant.values,
                  selected: variant,
                  label: (v) => v.label,
                  onSelect: (v) => setState(() => variant = v),
                ),
                const SizedBox(height: 20),
                _ChipSection<int>(
                  title: 'BOARD SIZE',
                  options: const [9, 13, 19],
                  selected: size,
                  label: (v) => '$v×$v',
                  onSelect: (v) => setState(() => size = v),
                ),
                const SizedBox(height: 20),
                _ChipSection<double>(
                  title: 'KOMI',
                  options: const [0.5, 5.5, 6.5, 7.0, 7.5, 8.0],
                  selected: komi,
                  label: (v) => v == v.roundToDouble()
                      ? v.toStringAsFixed(0)
                      : v.toString(),
                  onSelect: (v) => setState(() => komi = v),
                ),
                const SizedBox(height: 20),
                const _SectionLabel('HANDICAP'),
                const SizedBox(height: 8),
                ..._handicapRows(),
                const SizedBox(height: 20),
                const _SectionLabel('TIME CONTROL'),
                const SizedBox(height: 8),
                SizedBox(
                  height: 44,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: _TimePreset.values.length,
                    separatorBuilder: (_, __) => const SizedBox(width: 8),
                    itemBuilder: (context, i) {
                      final preset = _TimePreset.values[i];
                      return SizedBox(
                        width: 110,
                        child: ZenOptionButton(
                          label: preset.label,
                          selected: preset == timePreset,
                          onTap: () => setState(() => timePreset = preset),
                        ),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  timePreset.control.describe(),
                  style: text.labelSmall
                      ?.copyWith(color: scheme.onSurfaceVariant),
                ),
                if (widget.isAi) ...[
                  const SizedBox(height: 20),
                  const _SectionLabel('OPPONENT'),
                  const SizedBox(height: 8),
                  _BotRow(
                    bot: selectedBot,
                    onTap: _pickBot,
                  ),
                  const SizedBox(height: 20),
                  const _SectionLabel('OPPONENT PLAYS'),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Expanded(
                        child: _optionButton(
                            'BLACK',
                            aiColor == StoneColor.black,
                            () => setState(() => aiColor = StoneColor.black)),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: _optionButton(
                            'WHITE',
                            aiColor == StoneColor.white,
                            () => setState(() => aiColor = StoneColor.white)),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 12),
          SizedBox(
            height: 60,
            child: FilledButton(
              onPressed: () => widget.onStart(GameSetup(
                config: GameConfig(
                  boardSize: size,
                  ruleset: ruleset,
                  komi: komi,
                  handicap: handicap,
                  allowSuicide: defaults.allowSuicide,
                  superkoMode: defaults.superkoMode,
                  variant: variant,
                ),
                opponent: widget.isAi ? Opponent.ai : Opponent.human,
                aiColor: aiColor,
                aiDifficulty: widget.isAi
                    ? (selectedBot?.engine ?? aiDifficulty)
                    : aiDifficulty,
                timeControl: timePreset.control,
                bot: widget.isAi ? selectedBot : null,
              )),
              child: Text('BEGIN GAME',
                  style: text.labelLarge?.copyWith(color: scheme.onPrimary)),
            ),
          ),
        ],
      ),
    );
  }

  List<Widget> _handicapRows() {
    const rows = <List<int>>[
      [0, 2, 3],
      [4, 5, 6],
      [7, 8, 9],
    ];
    final widgets = <Widget>[];
    for (var i = 0; i < rows.length; i++) {
      widgets.add(Row(
        children: [
          for (var j = 0; j < rows[i].length; j++) ...[
            Expanded(
              child: _optionButton('${rows[i][j]}', handicap == rows[i][j],
                  () => setState(() => handicap = rows[i][j])),
            ),
            if (j != rows[i].length - 1) const SizedBox(width: 8),
          ]
        ],
      ));
      if (i != rows.length - 1) widgets.add(const SizedBox(height: 8));
    }
    return widgets;
  }

  Widget _optionButton(String label, bool selected, VoidCallback onTap) =>
      ZenOptionButton(label: label, selected: selected, onTap: onTap);

  Future<void> _pickBot() async {
    final picked = await Navigator.of(context).push<BotProfile>(
      MaterialPageRoute(
        builder: (_) => BotPickerScreen(
          selected: selectedBot,
        ),
      ),
    );
    if (picked != null) {
      setState(() {
        selectedBot = picked;
        aiDifficulty = picked.engine;
      });
    }
  }
}

class _BotRow extends StatelessWidget {
  final BotProfile? bot;
  final VoidCallback onTap;
  const _BotRow({required this.bot, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final b = bot;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(14),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: scheme.surface,
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Text(b?.initials ?? '??',
                  style: text.labelLarge
                      ?.copyWith(fontWeight: FontWeight.w700)),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(b?.name ?? 'Pick an opponent',
                      style: text.bodyMedium
                          ?.copyWith(fontWeight: FontWeight.w600)),
                  if (b != null)
                    Text(
                      '${b.countryEmoji} · ${b.rankLabel} · ${b.style.label}',
                      style: text.labelSmall
                          ?.copyWith(color: scheme.onSurfaceVariant),
                    ),
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
          ],
        ),
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel(this.text);
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Text(text,
        style: Theme.of(context)
            .textTheme
            .labelSmall
            ?.copyWith(color: scheme.onSurfaceVariant));
  }
}

class _RulesetSection extends StatelessWidget {
  final Ruleset selected;
  final ValueChanged<Ruleset> onSelect;
  const _RulesetSection({required this.selected, required this.onSelect});

  // Compact label for the chip (the canonical [Ruleset.label] is "New Zealand";
  // the chip uses "NZ" to fit the row).
  static String _chipLabel(Ruleset r) =>
      r == Ruleset.newZealand ? 'NZ' : r.label;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const _SectionLabel('RULESET'),
        const SizedBox(height: 8),
        SizedBox(
          height: 44,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemCount: Ruleset.values.length,
            separatorBuilder: (_, __) => const SizedBox(width: 8),
            itemBuilder: (context, i) {
              final r = Ruleset.values[i];
              return SizedBox(
                width: r == Ruleset.trompTaylor ? 130 : 96,
                child: ZenOptionButton(
                  label: _chipLabel(r),
                  selected: r == selected,
                  onTap: () => onSelect(r),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _ChipSection<T> extends StatelessWidget {
  final String title;
  final List<T> options;
  final T selected;
  final String Function(T) label;
  final ValueChanged<T> onSelect;

  const _ChipSection({
    super.key,
    required this.title,
    required this.options,
    required this.selected,
    required this.label,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionLabel(title),
        const SizedBox(height: 8),
        Row(
          children: [
            for (var i = 0; i < options.length; i++) ...[
              Expanded(
                child: ZenOptionButton(
                  label: label(options[i]),
                  selected: options[i] == selected,
                  onTap: () => onSelect(options[i]),
                ),
              ),
              if (i != options.length - 1) const SizedBox(width: 8),
            ],
          ],
        ),
      ],
    );
  }
}
