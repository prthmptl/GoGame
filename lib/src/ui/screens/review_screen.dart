import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

import '../../data/saved_game_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/rules.dart';
import '../../domain/scoring.dart';
import '../../sgf/sgf_import.dart';
import '../board/board_canvas.dart';
import '../board/mini_stone.dart';
import '../components/zen_components.dart';

class ReviewScreen extends StatefulWidget {
  final String? savedGameId;
  final SavedGameRepo? repo;
  final SettingsStore settings;

  const ReviewScreen({
    super.key,
    this.savedGameId,
    this.repo,
    required this.settings,
  });

  @override
  State<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends State<ReviewScreen> {
  GameState? _loaded;
  String? _sgfText;
  String? _opponentStyle;
  String? _resultLabel;
  double? _blackTotal;
  double? _whiteTotal;
  int _index = 0;

  @override
  void initState() {
    super.initState();
    widget.settings.addListener(_onSettings);
    if (widget.savedGameId != null && widget.repo != null) {
      _loadSavedGame();
    }
  }

  @override
  void dispose() {
    widget.settings.removeListener(_onSettings);
    super.dispose();
  }

  void _onSettings() => setState(() {});

  Future<void> _loadSavedGame() async {
    final entity = await widget.repo!.get(widget.savedGameId!);
    final path = entity?.sgfPath;
    if (path != null && path.isNotEmpty) {
      try {
        final text = await File(path).readAsString();
        final state = SgfImport.import(text);
        if (!mounted) return;
        setState(() {
          _sgfText = text;
          _loaded = state;
          _opponentStyle = _opponentStyleFromLabel(entity?.opponentLabel);
          _resultLabel = (entity?.resultLabel.isNotEmpty ?? false)
              ? entity!.resultLabel
              : _resultFromSgf(text);
          _blackTotal = entity?.blackTotal;
          _whiteTotal = entity?.whiteTotal;
          _index = state.history.length;
        });
      } catch (_) {
        // ignore corrupt SGF
      }
    }
  }

  Future<void> _pickSgf() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.any,
      withData: true,
    );
    if (picked == null) return;
    String text;
    final f = picked.files.first;
    if (f.bytes != null) {
      text = String.fromCharCodes(f.bytes!);
    } else if (f.path != null) {
      text = await File(f.path!).readAsString();
    } else {
      return;
    }
    if (!mounted) return;
    final state = SgfImport.import(text);
    setState(() {
      _sgfText = text;
      _loaded = state;
      _opponentStyle = null;
      _resultLabel = _resultFromSgf(text);
      _blackTotal = null;
      _whiteTotal = null;
      _index = state.history.length;
    });
  }

  String? _opponentStyleFromLabel(String? label) {
    if (label == null) return null;
    for (final difficulty in AiDifficulty.values) {
      if (label.toLowerCase().contains(difficulty.label.toLowerCase())) {
        return difficulty.label;
      }
    }
    return null;
  }

  String? _resultFromSgf(String text) =>
      RegExp(r'RE\[([^\]]+)]').firstMatch(text)?.group(1);

  ScoreResult? _matchingFallbackScore(GameState state) {
    final result = _resultLabel;
    if (result == null || result.isEmpty) return null;
    final score = Scoring.score(state);
    return score.resultString == result ? score : null;
  }

  Future<void> _exportSgf() async {
    final text = _sgfText;
    if (text == null) return;
    final dir = await getTemporaryDirectory();
    final f = File(
        p.join(dir.path, 'game-${DateTime.now().millisecondsSinceEpoch}.sgf'));
    await f.writeAsString(text);
    // Surface a snackbar with the path so the user knows where it was written.
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Saved SGF to ${f.path}')),
    );
  }

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

  GameState _replay(GameConfig config, GameState source, int upTo) {
    var s = GameState.newGame(config);
    final moves = source.history.take(upTo);
    for (final m in moves) {
      final intent = switch (m.type) {
        MoveType.pass => const MoveIntent.pass(),
        MoveType.resign => const MoveIntent.resign(),
        MoveType.placeStone => MoveIntent.place(m.point!),
      };
      final r = Rules.apply(s, intent);
      if (!r.isAccepted) break;
      s = r.newStateAs<GameState>();
    }
    return s;
  }

  Map<Point, int> _moveNumberMap(GameState state) {
    final out = <Point, int>{};
    for (final m in state.history) {
      if (m.type == MoveType.placeStone &&
          m.point != null &&
          state.board.cellAt(m.point!) != CellState.empty) {
        out[m.point!] = m.moveNumber;
      }
    }
    return out;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final loaded = _loaded;
    if (loaded == null) {
      return SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: ZenCard(
          container: scheme.surfaceContainerLow,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('Review a game',
                  style: text.headlineSmall
                      ?.copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 12),
              Text(
                'Import an SGF file to step through it move by move. '
                'Finished games are also saved here automatically — tap one on the home screen to open it.',
                style:
                    text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
              ),
              const SizedBox(height: 12),
              SizedBox(
                height: 56,
                child: FilledButton.icon(
                  onPressed: _pickSgf,
                  icon: const Icon(Icons.file_open),
                  label: Text('IMPORT SGF',
                      style:
                          text.labelLarge?.copyWith(color: scheme.onPrimary)),
                ),
              ),
            ],
          ),
        ),
      );
    }

    final total = loaded.history.length;
    final moveIndex = _index.clamp(0, total);
    final replayed = _replay(loaded.config, loaded, moveIndex);
    final moveNumbers = widget.settings.value.showMoveNumbers
        ? _moveNumberMap(replayed)
        : const <Point, int>{};
    final fallbackScore =
        (_blackTotal == null || _whiteTotal == null)
            ? _matchingFallbackScore(loaded)
            : null;
    final reviewBlackTotal = _blackTotal ?? fallbackScore?.blackTotal;
    final reviewWhiteTotal = _whiteTotal ?? fallbackScore?.whiteTotal;

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ZenCard(
            container: scheme.surfaceContainerLow,
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Replay',
                          style: text.labelMedium
                              ?.copyWith(color: scheme.onSurfaceVariant)),
                      Text('Move $moveIndex / $total',
                          style: text.headlineSmall
                              ?.copyWith(fontWeight: FontWeight.w600)),
                    ],
                  ),
                ),
                if (_sgfText != null)
                  IconButton(
                    onPressed: _exportSgf,
                    icon: const Icon(Icons.file_download),
                    tooltip: 'Export SGF',
                  ),
                IconButton(
                  onPressed: () => setState(() {
                    _loaded = null;
                    _sgfText = null;
                    _opponentStyle = null;
                    _resultLabel = null;
                    _blackTotal = null;
                    _whiteTotal = null;
                  }),
                  icon: const Icon(Icons.file_open),
                  tooltip: 'Open another',
                ),
              ],
            ),
          ),
          const SizedBox(height: 10),
          AspectRatio(
            aspectRatio: 1,
            child: BoardCanvas(
              board: replayed.board,
              overlay: BoardOverlay(
                lastMove: replayed.lastMove?.point,
                moveNumbers: moveNumbers,
              ),
              appearance: _appearance(),
            ),
          ),
          Slider(
            value: moveIndex.toDouble(),
            onChanged: (v) => setState(() => _index = v.toInt()),
            min: 0,
            max: total.toDouble().clamp(1, double.infinity),
            divisions: total > 0 ? total : null,
          ),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceEvenly,
            children: [
              IconButton(
                  onPressed: () => setState(() => _index = 0),
                  icon: const Icon(Icons.first_page)),
              IconButton(
                onPressed: _index > 0 ? () => setState(() => _index--) : null,
                icon: const Icon(Icons.navigate_before),
              ),
              IconButton(
                onPressed:
                    _index < total ? () => setState(() => _index++) : null,
                icon: const Icon(Icons.navigate_next),
              ),
              IconButton(
                  onPressed: () => setState(() => _index = total),
                  icon: const Icon(Icons.last_page)),
            ],
          ),
          const SizedBox(height: 10),
          _ReviewSummaryCard(
            config: loaded.config,
            totalMoves: total,
            opponentStyle: _opponentStyle,
            resultLabel: _resultLabel,
            blackTotal: reviewBlackTotal,
            whiteTotal: reviewWhiteTotal,
          ),
        ],
      ),
    );
  }
}

class _ReviewSummaryCard extends StatelessWidget {
  final GameConfig config;
  final int totalMoves;
  final String? opponentStyle;
  final String? resultLabel;
  final double? blackTotal;
  final double? whiteTotal;

  const _ReviewSummaryCard({
    required this.config,
    required this.totalMoves,
    required this.opponentStyle,
    required this.resultLabel,
    required this.blackTotal,
    required this.whiteTotal,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final outcome = _ReviewOutcome.fromLabel(resultLabel);
    final hasTotals = blackTotal != null && whiteTotal != null;

    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Final result',
                        style: text.labelMedium
                            ?.copyWith(color: scheme.onSurfaceVariant)),
                    Text(
                      outcome.headline,
                      style: text.headlineMedium
                          ?.copyWith(fontWeight: FontWeight.w700),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      outcome.detail,
                      style: text.bodyMedium
                          ?.copyWith(color: scheme.onSurfaceVariant),
                    ),
                  ],
                ),
              ),
              if (resultLabel != null && resultLabel!.isNotEmpty)
                ZenChip(
                  text: resultLabel!,
                  container: scheme.primaryContainer,
                  contentColor: scheme.onPrimaryContainer,
                ),
            ],
          ),
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHigh,
              borderRadius: BorderRadius.circular(16),
            ),
            child: hasTotals
                ? Row(
                    children: [
                      Expanded(
                        child: _ScoreTotal(
                          color: StoneColor.black,
                          value: blackTotal!,
                        ),
                      ),
                      Container(
                        width: 1,
                        height: 42,
                        color: scheme.outlineVariant,
                      ),
                      Expanded(
                        child: _ScoreTotal(
                          color: StoneColor.white,
                          value: whiteTotal!,
                        ),
                      ),
                    ],
                  )
                : Text(
                    _totalsUnavailableText(resultLabel),
                    style: text.bodyMedium
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              ZenChip(text: '${config.boardSize}×${config.boardSize}'),
              ZenChip(text: config.ruleset.label),
              ZenChip(text: 'Komi ${config.komi.toStringAsFixed(1)}'),
              ZenChip(text: '$totalMoves moves'),
              if (opponentStyle != null)
                ZenChip(text: '$opponentStyle style'),
            ],
          ),
        ],
      ),
    );
  }

  String _totalsUnavailableText(String? label) {
    final match = RegExp(r'^[BW]\+(.+)$').firstMatch(label ?? '');
    final suffix = match?.group(1)?.toUpperCase();
    return switch (suffix) {
      'R' => 'No point total — this game ended by resignation.',
      'T' => 'No point total — this game ended on time.',
      _ => 'Point totals are unavailable for this record.',
    };
  }
}

class _ScoreTotal extends StatelessWidget {
  final StoneColor color;
  final double value;

  const _ScoreTotal({
    required this.color,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    final label = color == StoneColor.black ? 'Black' : 'White';
    return Column(
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            MiniStone(color: color, size: 18),
            const SizedBox(width: 8),
            Text(
              label,
              style:
                  text.labelMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          _formatPoints(value),
          style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w700),
        ),
      ],
    );
  }
}

class _ReviewOutcome {
  final String headline;
  final String detail;

  const _ReviewOutcome(this.headline, this.detail);

  factory _ReviewOutcome.fromLabel(String? label) {
    final value = label?.trim();
    if (value == null || value.isEmpty) {
      return const _ReviewOutcome(
        'Result unavailable',
        'No final result was recorded for this game.',
      );
    }
    if (value.toLowerCase() == 'draw') {
      return const _ReviewOutcome('Draw', 'Both players finished level.');
    }

    final match = RegExp(r'^([BW])\+(.+)$').firstMatch(value);
    if (match == null) {
      return _ReviewOutcome(value, 'Final result');
    }

    final winner = match.group(1) == 'B' ? 'Black' : 'White';
    final suffix = match.group(2)!;
    final detail = switch (suffix.toUpperCase()) {
      'R' => 'by resignation',
      'T' => 'on time',
      _ => _numericMarginDetail(suffix),
    };
    return _ReviewOutcome('$winner wins', detail);
  }

  static String _numericMarginDetail(String value) {
    final margin = double.tryParse(value);
    if (margin == null) return 'Final result';
    return 'by ${_formatPoints(margin)} points';
  }
}

String _formatPoints(double value) =>
    value == value.roundToDouble()
        ? value.toStringAsFixed(0)
        : value.toStringAsFixed(1);
