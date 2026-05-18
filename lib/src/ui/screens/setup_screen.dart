import 'package:flutter/material.dart';

import '../../domain/models.dart';
import '../components/zen_components.dart';
import 'game_view_model.dart';

class GameSetup {
  final GameConfig config;
  final Opponent opponent;
  final StoneColor aiColor;
  final AiDifficulty aiDifficulty;
  const GameSetup(
      {required this.config,
      required this.opponent,
      required this.aiColor,
      required this.aiDifficulty});
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
                if (widget.isAi) ...[
                  const SizedBox(height: 20),
                  _ChipSection<AiDifficulty>(
                    title: 'OPPONENT STYLE',
                    options: AiDifficulty.values,
                    selected: aiDifficulty,
                    label: (v) => v.label,
                    onSelect: (v) => setState(() => aiDifficulty = v),
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
                ),
                opponent: widget.isAi ? Opponent.ai : Opponent.human,
                aiColor: aiColor,
                aiDifficulty: aiDifficulty,
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
