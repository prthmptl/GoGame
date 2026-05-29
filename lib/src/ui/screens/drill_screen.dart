import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../data/drill_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/ai/advanced_ai.dart';
import '../../domain/ai/beginner_ai.dart';
import '../../domain/ai/go_ai.dart';
import '../../domain/ai/intermediate_ai.dart';
import '../../domain/board.dart';
import '../../domain/drills/drill.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/rules.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

class DrillScreen extends StatefulWidget {
  final Drill drill;
  final DrillRepo repo;
  final SettingsStore settings;
  const DrillScreen({
    super.key,
    required this.drill,
    required this.repo,
    required this.settings,
  });

  @override
  State<DrillScreen> createState() => _DrillScreenState();
}

class _DrillScreenState extends State<DrillScreen> {
  late GameState _state;
  late GoAi _ai;
  bool _aiThinking = false;
  String? _result;
  bool _recorded = false;

  @override
  void initState() {
    super.initState();
    _ai = _buildAi(widget.drill.aiLevel);
    _state = _seed(widget.drill);
    _scheduleAiIfNeeded();
  }

  GoAi _buildAi(AiDifficulty d) => switch (d) {
        AiDifficulty.beginner => BeginnerAi(),
        AiDifficulty.intermediate => IntermediateAi(),
        AiDifficulty.advanced => AdvancedAi(),
      };

  GameState _seed(Drill drill) {
    var board = Board.empty(drill.boardSize);
    board = board.setMany([
      for (final p in drill.initialBlack) MapEntry(p, CellState.black),
      for (final p in drill.initialWhite) MapEntry(p, CellState.white),
    ]);
    final config = GameConfig(
      boardSize: drill.boardSize,
      variant: GameVariant.atariGo,
    );
    final seedHash =
        Rules.positionHash(config.superkoMode, board, drill.playerColor);
    return GameState(
      board: board,
      config: config,
      currentPlayer: drill.playerColor,
      moveNumber: 0,
      capturesByBlack: 0,
      capturesByWhite: 0,
      koPoint: null,
      previousHashes: <int>{seedHash},
      status: GameStatus.active,
      consecutivePasses: 0,
      lastMove: null,
      history: const [],
    );
  }

  void _onTap(Point point) {
    if (_state.status != GameStatus.active) return;
    if (_state.currentPlayer != widget.drill.playerColor) return;
    final res = Rules.apply(_state, MoveIntent.place(point));
    if (!res.isAccepted) return;
    HapticFeedback.lightImpact();
    final next = res.newStateAs<GameState>();
    setState(() => _state = next);
    _checkComplete();
    _scheduleAiIfNeeded();
  }

  void _scheduleAiIfNeeded() {
    if (_state.status != GameStatus.active) return;
    if (_state.currentPlayer == widget.drill.playerColor) return;
    setState(() => _aiThinking = true);
    final snapshot = _state;
    Future<void>(() async {
      await Future<void>.delayed(const Duration(milliseconds: 200));
      final intent = _ai.chooseMove(snapshot);
      if (!mounted) return;
      final res = Rules.apply(_state, intent);
      if (!res.isAccepted) {
        setState(() => _aiThinking = false);
        return;
      }
      setState(() {
        _state = res.newStateAs<GameState>();
        _aiThinking = false;
      });
      _checkComplete();
    });
  }

  void _checkComplete() {
    if (_state.status != GameStatus.completed) return;
    if (_recorded) return;
    _recorded = true;
    final winner = _state.history.last.player;
    final playerWon = winner == widget.drill.playerColor;
    if (playerWon) {
      widget.repo.recordWin(widget.drill);
      setState(() => _result = 'You captured!');
    } else {
      widget.repo.recordLoss(widget.drill);
      setState(() => _result = 'AI captured first.');
    }
  }

  void _retry() {
    setState(() {
      _state = _seed(widget.drill);
      _aiThinking = false;
      _result = null;
      _recorded = false;
    });
    _scheduleAiIfNeeded();
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
    final wins = widget.repo.winsFor(widget.drill.id);
    final mastered = widget.repo.isMastered(widget.drill.id);
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: Text(widget.drill.title),
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
                  Text(widget.drill.description, style: text.bodyLarge),
                  const SizedBox(height: 8),
                  Text(
                    mastered
                        ? 'You’ve mastered this drill.'
                        : 'Streak: $wins / ${widget.drill.requiredWins}',
                    style: text.labelMedium
                        ?.copyWith(color: scheme.onSurfaceVariant),
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
                  overlay: BoardOverlay(
                    lastMove: _state.lastMove?.point,
                  ),
                  appearance: _appearance(),
                  onTap: _onTap,
                ),
              ),
            ),
            const SizedBox(height: 12),
            if (_result != null)
              ZenCard(
                container: _result!.startsWith('You')
                    ? scheme.primary.withValues(alpha: 0.12)
                    : scheme.error.withValues(alpha: 0.12),
                child: Text(_result!, style: text.headlineSmall),
              ),
            if (_aiThinking) ...[
              const SizedBox(height: 8),
              Text('Opponent is thinking…',
                  style:
                      text.labelMedium?.copyWith(color: scheme.onSurfaceVariant)),
            ],
            const SizedBox(height: 12),
            SizedBox(
              height: 56,
              child: FilledButton.icon(
                onPressed: _retry,
                icon: const Icon(Icons.refresh),
                label: Text(_result == null ? 'RESTART' : 'PLAY AGAIN'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
