import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../data/settings_store.dart';
import '../../domain/ai/ai_heuristics.dart';
import '../../domain/ai/beginner_ai.dart';
import '../../domain/ai/go_ai.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/rules.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

/// Free-exploration board: alternate stones at will, ask the engine for
/// suggestions, undo, reset, and switch board size.
class SandboxScreen extends StatefulWidget {
  final SettingsStore settings;
  const SandboxScreen({super.key, required this.settings});

  @override
  State<SandboxScreen> createState() => _SandboxScreenState();
}

class _SandboxScreenState extends State<SandboxScreen> {
  int _size = 9;
  late GameState _state;
  Point? _suggestion;
  String? _evalSummary;
  final GoAi _ai = BeginnerAi();

  @override
  void initState() {
    super.initState();
    _state = GameState.newGame(GameConfig(boardSize: _size));
  }

  void _tap(Point point) {
    if (_state.board.cellAt(point) != CellState.empty) return;
    final res = Rules.apply(_state, MoveIntent.place(point));
    if (!res.isAccepted) return;
    HapticFeedback.lightImpact();
    setState(() {
      _state = res.newStateAs<GameState>();
      _suggestion = null;
      _evalSummary = null;
    });
  }

  void _pass() {
    final res = Rules.apply(_state, const MoveIntent.pass());
    if (!res.isAccepted) return;
    setState(() {
      _state = res.newStateAs<GameState>();
      _suggestion = null;
      _evalSummary = null;
    });
  }

  void _undo() {
    if (_state.history.isEmpty) return;
    final replay = _state.history.sublist(0, _state.history.length - 1);
    var s = GameState.newGame(GameConfig(boardSize: _size));
    for (final m in replay) {
      final intent = switch (m.type) {
        MoveType.pass => const MoveIntent.pass(),
        MoveType.resign => const MoveIntent.resign(),
        MoveType.placeStone => MoveIntent.place(m.point!),
      };
      final r = Rules.apply(s, intent);
      if (r.isAccepted) s = r.newStateAs<GameState>();
    }
    setState(() {
      _state = s;
      _suggestion = null;
      _evalSummary = null;
    });
  }

  void _reset({int? size}) {
    setState(() {
      if (size != null) _size = size;
      _state = GameState.newGame(GameConfig(boardSize: _size));
      _suggestion = null;
      _evalSummary = null;
    });
  }

  void _askEngine() {
    final intent = _ai.chooseMove(_state);
    final eval =
        AiHeuristics.evaluate(_state, _state.currentPlayer);
    setState(() {
      _suggestion = intent.type == MoveType.placeStone ? intent.point : null;
      _evalSummary =
          intent.type == MoveType.pass ? 'Engine suggests pass.' : null;
      _evalSummary ??=
          'Eval for ${_state.currentPlayer == StoneColor.black ? 'Black' : 'White'}: $eval';
    });
  }

  BoardAppearance _appearance() {
    final base = switch (widget.settings.value.boardTheme) {
      BoardThemeKind.classicWood => BoardAppearance.classicWood,
      BoardThemeKind.minimalPaper => BoardAppearance.minimalPaper,
      BoardThemeKind.darkSlate => BoardAppearance.darkSlate,
      BoardThemeKind.highContrast => BoardAppearance.highContrast,
    };
    return base.copyWith(
        showCoordinates: widget.settings.value.showCoordinates);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final overlay = BoardOverlay(
      lastMove: _state.lastMove?.point,
      markers: _suggestion == null ? const {} : {_suggestion!},
    );
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Analysis sandbox'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
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
                  Text(
                    'Free-explore mode. Stones alternate. Ask the engine for a suggestion any time.',
                    style: text.bodyMedium
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      for (final s in const [9, 13, 19]) ...[
                        Expanded(
                          child: ZenOptionButton(
                            label: '$s×$s',
                            selected: _size == s,
                            onTap: () => _reset(size: s),
                          ),
                        ),
                        if (s != 19) const SizedBox(width: 8),
                      ],
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
            AspectRatio(
              aspectRatio: 1,
              child: ZenCard(
                contentPadding: const EdgeInsets.all(6),
                child: BoardCanvas(
                  board: _state.board,
                  overlay: overlay,
                  appearance: _appearance(),
                  onTap: _tap,
                ),
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _state.history.isEmpty ? null : _undo,
                    icon: const Icon(Icons.undo),
                    label: const Text('Undo'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _pass,
                    icon: const Icon(Icons.skip_next),
                    label: const Text('Pass'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _reset,
                    icon: const Icon(Icons.refresh),
                    label: const Text('Reset'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            SizedBox(
              height: 52,
              child: FilledButton.icon(
                onPressed: _askEngine,
                icon: const Icon(Icons.auto_awesome),
                label: const Text('SUGGEST A MOVE'),
              ),
            ),
            if (_evalSummary != null) ...[
              const SizedBox(height: 10),
              ZenCard(
                container: scheme.surfaceContainerLow,
                child: Text(_evalSummary!, style: text.bodyMedium),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
