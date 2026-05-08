import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:share_plus/share_plus.dart';

import '../../data/settings_store.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/scoring.dart';
import '../board/board_canvas.dart';
import '../board/mini_stone.dart';
import '../components/zen_components.dart';
import 'game_view_model.dart';

class GameScreen extends StatefulWidget {
  final GameViewModel vm;
  final SettingsStore settings;
  final VoidCallback onExit;

  const GameScreen({
    super.key,
    required this.vm,
    required this.settings,
    required this.onExit,
  });

  @override
  State<GameScreen> createState() => _GameScreenState();
}

class _GameScreenState extends State<GameScreen> {
  @override
  void initState() {
    super.initState();
    widget.vm.addListener(_onVm);
    widget.settings.addListener(_onSettings);
  }

  @override
  void dispose() {
    widget.vm.removeListener(_onVm);
    widget.settings.removeListener(_onSettings);
    super.dispose();
  }

  void _onVm() => setState(() {});
  void _onSettings() => setState(() {});

  BoardAppearance _appearance() {
    final s = widget.settings.value;
    final base = switch (s.boardTheme) {
      BoardThemeKind.classicWood => BoardAppearance.classicWood,
      BoardThemeKind.minimalPaper => BoardAppearance.minimalPaper,
      BoardThemeKind.darkSlate => BoardAppearance.darkSlate,
      BoardThemeKind.highContrast => BoardAppearance.highContrast,
    };
    return base.copyWith(showCoordinates: s.showCoordinates);
  }

  StoneColor _humanColor(GameUi ui) =>
      ui.opponent == Opponent.ai ? ui.aiPlays.other : StoneColor.black;

  bool _lowTime(GameUi ui, StoneColor color) {
    final ms = color == StoneColor.black ? ui.blackMillis : ui.whiteMillis;
    return ui.state.status == GameStatus.active && ms > 0 && ms <= 30000;
  }

  Future<void> _confirmResign() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Resign?'),
        content: const Text('This will end the game.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancel')),
          TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Resign')),
        ],
      ),
    );
    if (confirmed == true) widget.vm.resign();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final ui = widget.vm.ui;
    final state = ui.state;

    return LayoutBuilder(builder: (context, constraints) {
      final boardSide =
          math.min(constraints.maxWidth, constraints.maxHeight * 0.58);
      return SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Center(
              child: ZenChip(
                container: scheme.primary,
                text: switch (state.status) {
                  GameStatus.active =>
                    ui.opponent == Opponent.ai ? 'VS AI' : 'LOCAL MATCH',
                  GameStatus.scoring => 'SCORING',
                  GameStatus.completed =>
                    ui.timeoutLoser != null ? 'TIMEOUT' : 'COMPLETED',
                  GameStatus.resigned => 'RESIGNED',
                },
              ),
            ),
            const SizedBox(height: 8),
            _PlayerCard(
              color: ui.opponent == Opponent.ai ? ui.aiPlays : StoneColor.white,
              name: ui.opponent == Opponent.ai ? 'AI' : 'Opponent',
              time: GameViewModel.formatTime(
                ((ui.opponent == Opponent.ai ? ui.aiPlays : StoneColor.white) ==
                        StoneColor.black)
                    ? ui.blackMillis
                    : ui.whiteMillis,
              ),
              active: state.status == GameStatus.active &&
                  state.currentPlayer ==
                      (ui.opponent == Opponent.ai
                          ? ui.aiPlays
                          : StoneColor.white),
              lowTime: _lowTime(ui,
                  ui.opponent == Opponent.ai ? ui.aiPlays : StoneColor.white),
            ),
            const SizedBox(height: 8),
            SizedBox(
              height: boardSide,
              child: Center(
                child: SizedBox(
                  width: boardSide,
                  height: boardSide,
                  child: BoardCanvas(
                    board: state.board,
                    overlay: BoardOverlay(
                      lastMove: state.lastMove?.point,
                      koPoint: state.koPoint,
                      deadStones: ui.deadStones,
                      pending: ui.pendingPoint == null
                          ? null
                          : (
                              point: ui.pendingPoint!,
                              color: state.currentPlayer
                            ),
                    ),
                    appearance: _appearance(),
                    onTap: widget.vm.tap,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 8),
            _PlayerCard(
              color: _humanColor(ui),
              name: 'You',
              time: GameViewModel.formatTime(
                _humanColor(ui) == StoneColor.black
                    ? ui.blackMillis
                    : ui.whiteMillis,
              ),
              active: state.status == GameStatus.active &&
                  state.currentPlayer == _humanColor(ui),
              lowTime: _lowTime(ui, _humanColor(ui)),
            ),
            if (ui.rejection != null) ...[
              const SizedBox(height: 8),
              Text('⚠ ${ui.rejection!}',
                  style: text.labelMedium?.copyWith(color: scheme.error)),
            ],
            if (ui.aiThinking) ...[
              const SizedBox(height: 8),
              Text('AI is thinking…',
                  style: text.labelMedium
                      ?.copyWith(color: scheme.onSurfaceVariant)),
            ],
            if (ui.pendingPoint != null &&
                state.status == GameStatus.active) ...[
              const SizedBox(height: 8),
              Text(
                  'Tap again to confirm — or tap elsewhere to choose a different point.',
                  style: text.labelMedium?.copyWith(color: scheme.primary)),
            ],
            const SizedBox(height: 12),
            _Controls(
              ui: ui,
              vm: widget.vm,
              onExit: widget.onExit,
              onResignTap: _confirmResign,
            ),
          ],
        ),
      );
    });
  }
}

class _PlayerCard extends StatelessWidget {
  final StoneColor color;
  final String name;
  final String time;
  final bool active;
  final bool lowTime;

  const _PlayerCard({
    required this.color,
    required this.name,
    required this.time,
    required this.active,
    required this.lowTime,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return ZenCard(
      container:
          active ? scheme.surfaceContainerHigh : scheme.surfaceContainerLow,
      child: Row(
        children: [
          MiniStone(color: color, size: 36),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name,
                    style:
                        text.bodyMedium?.copyWith(fontWeight: FontWeight.w600)),
                Text(active ? 'Thinking…' : 'Waiting',
                    style: text.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant)),
              ],
            ),
          ),
          Text(
            time,
            style: text.headlineSmall?.copyWith(
              fontWeight: FontWeight.bold,
              color: lowTime
                  ? scheme.error
                  : active
                      ? scheme.primary
                      : scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

class _Controls extends StatelessWidget {
  final GameUi ui;
  final GameViewModel vm;
  final VoidCallback onExit;
  final VoidCallback onResignTap;

  const _Controls({
    required this.ui,
    required this.vm,
    required this.onExit,
    required this.onResignTap,
  });

  @override
  Widget build(BuildContext context) {
    switch (ui.state.status) {
      case GameStatus.active:
        return _ActiveControls(
          onPass: vm.pass,
          onUndo: vm.undo,
          onResign: onResignTap,
          onExit: onExit,
        );
      case GameStatus.scoring:
        return _ScoringControls(
          score: ui.score,
          onConfirm: vm.confirmScore,
          onResume: vm.resumePlay,
        );
      case GameStatus.completed:
      case GameStatus.resigned:
        return _CompletedControls(
          state: ui.state,
          score: ui.score,
          timedOut: ui.timeoutLoser,
          onExportSgf: () async {
            final sgf = vm.exportSgf();
            await Share.share(sgf, subject: 'Go game (SGF)');
          },
          onExit: onExit,
        );
    }
  }
}

class _ActiveControls extends StatelessWidget {
  final VoidCallback onPass;
  final VoidCallback onUndo;
  final VoidCallback onResign;
  final VoidCallback onExit;

  const _ActiveControls({
    required this.onPass,
    required this.onUndo,
    required this.onResign,
    required this.onExit,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        Row(
          children: [
            Expanded(
              child: SizedBox(
                height: 52,
                child: OutlinedButton(
                  onPressed: onPass,
                  child: Text('PASS', style: text.labelLarge),
                ),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: SizedBox(
                height: 52,
                child: OutlinedButton(
                  onPressed: onUndo,
                  child: Text('UNDO', style: text.labelLarge),
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        SizedBox(
          width: double.infinity,
          height: 56,
          child: FilledButton(
            onPressed: onResign,
            child: Text('RESIGN',
                style: text.labelLarge?.copyWith(color: scheme.onPrimary)),
          ),
        ),
        TextButton(onPressed: onExit, child: const Text('Exit to menu')),
      ],
    );
  }
}

class _ScoringControls extends StatelessWidget {
  final ScoreResult? score;
  final VoidCallback onConfirm;
  final VoidCallback onResume;

  const _ScoringControls({
    required this.score,
    required this.onConfirm,
    required this.onResume,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    return ZenCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Tap stones to mark them dead',
              style: text.bodyMedium?.copyWith(fontWeight: FontWeight.w600)),
          if (score != null) ...[
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('Black area  ${score!.blackArea}', style: text.bodyMedium),
                Text('White total  ${score!.whiteTotal.toStringAsFixed(1)}',
                    style: text.bodyMedium),
              ],
            ),
            const SizedBox(height: 4),
            Text(score!.resultString,
                style: text.headlineMedium?.copyWith(
                    fontWeight: FontWeight.bold, color: scheme.primary)),
          ],
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: SizedBox(
                  height: 56,
                  child: OutlinedButton(
                      onPressed: onResume,
                      child: Text('RESUME', style: text.labelLarge)),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: SizedBox(
                  height: 56,
                  child: FilledButton(
                    onPressed: onConfirm,
                    child: Text('CONFIRM',
                        style:
                            text.labelLarge?.copyWith(color: scheme.onPrimary)),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _CompletedControls extends StatelessWidget {
  final GameState state;
  final ScoreResult? score;
  final StoneColor? timedOut;
  final VoidCallback onExportSgf;
  final VoidCallback onExit;

  const _CompletedControls({
    required this.state,
    required this.score,
    required this.timedOut,
    required this.onExportSgf,
    required this.onExit,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    return ZenCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (timedOut != null)
            Text(
                '${timedOut!.other == StoneColor.black ? 'BLACK' : 'WHITE'} wins on time',
                style: text.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold, color: scheme.primary))
          else if (state.status == GameStatus.resigned &&
              state.history.isNotEmpty)
            Text(
              '${state.history.last.player.other == StoneColor.black ? 'BLACK' : 'WHITE'} wins by resignation',
              style: text.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold, color: scheme.primary),
            )
          else if (score != null) ...[
            Text(score!.resultString,
                style: text.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold, color: scheme.primary)),
            const SizedBox(height: 4),
            Text('Black',
                style:
                    text.labelSmall?.copyWith(color: scheme.onSurfaceVariant)),
            Text(
              'Stones ${score!.blackStones}  ·  Territory ${score!.blackTerritory}  =  ${score!.blackArea}',
              style: text.bodyMedium,
            ),
            const SizedBox(height: 4),
            Text('White',
                style:
                    text.labelSmall?.copyWith(color: scheme.onSurfaceVariant)),
            Text(
              'Stones ${score!.whiteStones}  ·  Territory ${score!.whiteTerritory}  ·  '
              'Komi ${score!.komi.toStringAsFixed(1)}  =  ${score!.whiteTotal.toStringAsFixed(1)}',
              style: text.bodyMedium,
            ),
          ],
          const SizedBox(height: 8),
          Text(
            'SGF (Smart Game Format) is the standard text format for sharing a game record.',
            style: text.labelSmall?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            height: 56,
            child: FilledButton(
              onPressed: onExportSgf,
              child: Text('SHARE SGF',
                  style: text.labelLarge?.copyWith(color: scheme.onPrimary)),
            ),
          ),
          TextButton(onPressed: onExit, child: const Text('Back to menu')),
        ],
      ),
    );
  }
}
