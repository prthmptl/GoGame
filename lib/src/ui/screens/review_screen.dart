import 'dart:io';
import 'dart:convert';
import 'dart:math' as math;

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show compute;
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

import '../../data/saved_game_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/game_state.dart';
import '../../domain/models.dart';
import '../../domain/review/review_analyzer.dart';
import '../../domain/review/variation_tree.dart';
import '../../domain/rules.dart';
import '../../domain/scoring.dart';
import '../../sgf/sgf_import.dart';
import '../../sgf/sgf_tree.dart';
import '../board/board_canvas.dart';
import '../board/mini_stone.dart';
import '../components/sparkline.dart';
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

  VariationTree? _tree;
  List<VariationNode> _path = const [];
  bool _exploreMode = false;

  ReviewReport? _report;
  bool _analyzing = false;

  int get _index => _path.isEmpty ? 0 : _path.length - 1;

  VariationNode? get _currentNode => _path.isEmpty ? null : _path.last;

  bool get _onMainLine {
    if (_path.length < 2) return true;
    var node = _tree?.root;
    for (var i = 1; i < _path.length; i++) {
      if (node == null || node.children.isEmpty) return false;
      if (!identical(node.children.first, _path[i])) return false;
      node = node.children.first;
    }
    return true;
  }

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
        final tree = _treeFor(text, state);
        if (!mounted) return;
        setState(() {
          _sgfText = text;
          _loaded = state;
          _tree = tree;
          _path = _mainLinePath(tree);
          _opponentStyle = _opponentStyleFromLabel(entity?.opponentLabel);
          _resultLabel = (entity?.resultLabel.isNotEmpty ?? false)
              ? entity!.resultLabel
              : _resultFromSgf(text);
          _blackTotal = entity?.blackTotal;
          _whiteTotal = entity?.whiteTotal;
          _report = null;
        });
      } catch (_) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('Could not open this saved SGF.')));
        }
      }
    }
  }

  VariationTree _treeFor(String sgfText, GameState fallback) {
    try {
      final sgfRoot = SgfTreeParser.parse(sgfText);
      return VariationTreeBuilder.build(sgfRoot);
    } catch (_) {
      return VariationTreeBuilder.fromHistory(fallback);
    }
  }

  List<VariationNode> _mainLinePath(VariationTree tree) {
    final path = <VariationNode>[tree.root];
    var node = tree.root;
    while (node.children.isNotEmpty) {
      node = node.children.first;
      path.add(node);
    }
    return path;
  }

  Future<void> _pickSgf() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.any,
      withData: true,
    );
    if (picked == null) return;
    try {
      String text;
      final f = picked.files.first;
      if (f.bytes != null) {
        text = utf8.decode(f.bytes!);
      } else if (f.path != null) {
        text = await File(f.path!).readAsString();
      } else {
        return;
      }
      if (!mounted) return;
      final state = SgfImport.import(text);
      final tree = _treeFor(text, state);
      setState(() {
        _sgfText = text;
        _loaded = state;
        _tree = tree;
        _path = _mainLinePath(tree);
        _opponentStyle = null;
        _resultLabel = _resultFromSgf(text);
        _blackTotal = null;
        _whiteTotal = null;
        _report = null;
      });
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
            content: Text('This file is not a supported Go SGF.')));
      }
    }
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

  void _goToStart() {
    final tree = _tree;
    if (tree == null) return;
    setState(() => _path = [tree.root]);
  }

  void _goToEnd() {
    final tree = _tree;
    if (tree == null) return;
    setState(() => _path = _mainLinePath(tree));
  }

  void _stepBack() {
    if (_path.length <= 1) return;
    setState(() => _path = _path.sublist(0, _path.length - 1));
  }

  void _stepForward() {
    final node = _currentNode;
    if (node == null || node.children.isEmpty) return;
    setState(() => _path = [..._path, node.children.first]);
  }

  void _jumpTo(int targetDepth) {
    final tree = _tree;
    if (tree == null) return;
    final clamped = targetDepth.clamp(0, _mainLineDepth(tree));
    final path = <VariationNode>[tree.root];
    var node = tree.root;
    for (var i = 0; i < clamped && node.children.isNotEmpty; i++) {
      node = node.children.first;
      path.add(node);
    }
    setState(() => _path = path);
  }

  void _jumpToSibling(VariationNode sibling) {
    if (_path.length < 2) return;
    setState(() => _path = [..._path.sublist(0, _path.length - 1), sibling]);
  }

  void _onBoardTap(Point point) {
    final tree = _tree;
    final node = _currentNode;
    if (tree == null || node == null) return;
    final state = tree.buildStateTo(node);
    if (state == null) return;
    if (state.board.cellAt(point) != CellState.empty) return;
    final probe = Rules.apply(state, MoveIntent.place(point));
    if (!probe.isAccepted) return;
    final move = probe.move!;
    final next = tree.addExploration(node, move);
    setState(() => _path = [..._path, next]);
  }

  int _mainLineDepth(VariationTree tree) {
    var depth = 0;
    var node = tree.root;
    while (node.children.isNotEmpty) {
      node = node.children.first;
      depth++;
    }
    return depth;
  }

  List<VariationNode> _siblingChoicesAtCurrent() {
    if (_path.length < 2) return const [];
    final parent = _path[_path.length - 2];
    if (parent.children.length <= 1) return const [];
    return parent.children;
  }

  Future<void> _runAnalysis() async {
    final loaded = _loaded;
    if (loaded == null || _analyzing) return;
    setState(() {
      _analyzing = true;
      _report = null;
    });
    try {
      final report = await compute(
          _analyzeGame, (game: loaded, initial: _tree?.initialState));
      if (!mounted || !identical(_loaded, loaded)) return;
      setState(() => _report = report);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Could not analyze this game.')));
      }
    } finally {
      if (mounted) setState(() => _analyzing = false);
    }
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

    final tree = _tree;
    final mainLineLength =
        tree == null ? loaded.history.length : _mainLineDepth(tree);
    final pathDepth = _index;
    final currentNode = _currentNode;
    final replayed = (tree != null && currentNode != null)
        ? (tree.buildStateTo(currentNode) ?? loaded)
        : _replay(loaded.config, loaded, pathDepth);
    final moveNumbers = widget.settings.value.showMoveNumbers
        ? _moveNumberMap(replayed)
        : const <Point, int>{};
    final fallbackScore = (_blackTotal == null || _whiteTotal == null)
        ? _matchingFallbackScore(loaded)
        : null;
    final reviewBlackTotal = _blackTotal ?? fallbackScore?.blackTotal;
    final reviewWhiteTotal = _whiteTotal ?? fallbackScore?.whiteTotal;

    final analysis = _report;
    final onMainLine = _onMainLine;
    MoveAnalysis? currentAnalysis;
    if (onMainLine &&
        analysis != null &&
        pathDepth > 0 &&
        pathDepth <= analysis.moves.length) {
      currentAnalysis = analysis.moves[pathDepth - 1];
    }
    final recommendedMarker = (currentAnalysis?.recommended != null)
        ? <Point>{currentAnalysis!.recommended!}
        : const <Point>{};
    final siblings = _siblingChoicesAtCurrent();
    final sliderMax = math.max(1, math.max(mainLineLength, pathDepth));

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
                      Text(onMainLine ? 'Replay' : 'Variation',
                          style: text.labelMedium
                              ?.copyWith(color: scheme.onSurfaceVariant)),
                      Text('Move $pathDepth / $mainLineLength',
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
                    _tree = null;
                    _path = const [];
                    _exploreMode = false;
                    _report = null;
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
                markers: recommendedMarker,
              ),
              appearance: _appearance(),
              onTap: _exploreMode ? _onBoardTap : null,
            ),
          ),
          Slider(
            value: pathDepth.toDouble(),
            onChanged: (v) => _jumpTo(v.toInt()),
            min: 0,
            max: sliderMax.toDouble(),
            divisions: sliderMax > 0 ? sliderMax : null,
          ),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceEvenly,
            children: [
              IconButton(
                  onPressed: _goToStart, icon: const Icon(Icons.first_page)),
              IconButton(
                onPressed: _index > 0 ? _stepBack : null,
                icon: const Icon(Icons.navigate_before),
              ),
              IconButton(
                onPressed: (currentNode?.children.isNotEmpty ?? false)
                    ? _stepForward
                    : null,
                icon: const Icon(Icons.navigate_next),
              ),
              IconButton(
                  onPressed: _goToEnd, icon: const Icon(Icons.last_page)),
              IconButton(
                tooltip: _exploreMode ? 'Stop exploring' : 'Try a move',
                onPressed: () => setState(() => _exploreMode = !_exploreMode),
                icon: Icon(_exploreMode ? Icons.edit_off : Icons.edit),
                color: _exploreMode ? scheme.primary : null,
              ),
            ],
          ),
          if (siblings.isNotEmpty) ...[
            const SizedBox(height: 8),
            _VariationsCard(
              siblings: siblings,
              boardSize: replayed.board.size,
              onSelect: _jumpToSibling,
            ),
          ],
          const SizedBox(height: 10),
          _AnalysisCard(
            report: analysis,
            analyzing: _analyzing,
            cursor: pathDepth - 1,
            currentMove: currentAnalysis,
            onAnalyze: _runAnalysis,
          ),
          const SizedBox(height: 10),
          _ReviewSummaryCard(
            config: loaded.config,
            totalMoves: mainLineLength,
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
              if (opponentStyle != null) ZenChip(text: '$opponentStyle style'),
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
              style: text.labelMedium?.copyWith(color: scheme.onSurfaceVariant),
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

String _formatPoints(double value) => value == value.roundToDouble()
    ? value.toStringAsFixed(0)
    : value.toStringAsFixed(1);

class _AnalysisCard extends StatelessWidget {
  final ReviewReport? report;
  final bool analyzing;
  final int cursor;
  final MoveAnalysis? currentMove;
  final VoidCallback onAnalyze;

  const _AnalysisCard({
    required this.report,
    required this.analyzing,
    required this.cursor,
    required this.currentMove,
    required this.onAnalyze,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    if (analyzing) {
      return ZenCard(
        container: scheme.surfaceContainerLow,
        child: Row(
          children: [
            const SizedBox(
              width: 22,
              height: 22,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text('Analyzing moves…',
                  style: text.bodyMedium
                      ?.copyWith(color: scheme.onSurfaceVariant)),
            ),
          ],
        ),
      );
    }
    final r = report;
    if (r == null) {
      return ZenCard(
        container: scheme.surfaceContainerLow,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('AI Review', style: text.headlineSmall),
            const SizedBox(height: 4),
            Text(
              'Run the local engine over the game to find blunders, mistakes, and better moves.',
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 12),
            SizedBox(
              height: 48,
              child: FilledButton.icon(
                onPressed: onAnalyze,
                icon: const Icon(Icons.auto_awesome),
                label: const Text('ANALYZE GAME'),
              ),
            ),
          ],
        ),
      );
    }
    final values = r.moves.map((m) => m.blackLead).toList(growable: false);
    final maxAbs =
        values.fold<double>(10, (acc, v) => v.abs() > acc ? v.abs() : acc);
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text('AI Review', style: text.headlineSmall),
              ),
              TextButton.icon(
                onPressed: onAnalyze,
                icon: const Icon(Icons.refresh, size: 18),
                label: const Text('Re-run'),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              _CountChip(
                  label: 'Blunders', value: r.blunders, color: scheme.error),
              const SizedBox(width: 8),
              _CountChip(
                  label: 'Mistakes',
                  value: r.mistakes,
                  color: const Color(0xFFB87E2A)),
              const SizedBox(width: 8),
              _CountChip(
                  label: 'Inaccuracies',
                  value: r.inaccuracies,
                  color: scheme.onSurfaceVariant),
            ],
          ),
          const SizedBox(height: 12),
          Sparkline(values: values, cursor: cursor, maxAbs: maxAbs),
          const SizedBox(height: 12),
          if (currentMove != null)
            _CurrentMoveRow(move: currentMove!)
          else
            Text(
              'Step through the moves to see per-move feedback.',
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
        ],
      ),
    );
  }
}

class _CountChip extends StatelessWidget {
  final String label;
  final int value;
  final Color color;

  const _CountChip({
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return Expanded(
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.10),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(label.toUpperCase(),
                style: text.labelSmall
                    ?.copyWith(color: color, letterSpacing: 1.2)),
            const SizedBox(height: 2),
            Text('$value', style: text.headlineSmall?.copyWith(color: color)),
          ],
        ),
      ),
    );
  }
}

class _CurrentMoveRow extends StatelessWidget {
  final MoveAnalysis move;
  const _CurrentMoveRow({required this.move});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final color = _qualityColor(move.quality, scheme);
    return Row(
      children: [
        MiniStone(color: move.player, size: 22),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${move.player == StoneColor.black ? 'Black' : 'White'} · Move ${move.moveNumber}',
                style: text.labelSmall?.copyWith(
                    color: scheme.onSurfaceVariant, letterSpacing: 1.1),
              ),
              const SizedBox(height: 2),
              Text(
                move.comment ?? _defaultComment(move),
                style: text.bodyMedium,
              ),
            ],
          ),
        ),
        const SizedBox(width: 8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.16),
            borderRadius: BorderRadius.circular(999),
          ),
          child: Text(
            move.quality.label,
            style: text.labelSmall?.copyWith(color: color),
          ),
        ),
      ],
    );
  }

  static String _defaultComment(MoveAnalysis move) {
    if (move.quality == MoveQuality.best) return 'Matched the AI top move.';
    if (move.pointsLost == 0) return 'Reasonable choice.';
    return '${move.pointsLost} points behind the AI top choice.';
  }

  static Color _qualityColor(MoveQuality q, ColorScheme scheme) => switch (q) {
        MoveQuality.best => scheme.primary,
        MoveQuality.good => scheme.onSurfaceVariant,
        MoveQuality.inaccuracy => const Color(0xFFB87E2A),
        MoveQuality.mistake => const Color(0xFFC65D1B),
        MoveQuality.blunder => scheme.error,
        MoveQuality.notApplicable => scheme.onSurfaceVariant,
      };
}

class _VariationsCard extends StatelessWidget {
  final List<VariationNode> siblings;
  final int boardSize;
  final ValueChanged<VariationNode> onSelect;

  const _VariationsCard({
    required this.siblings,
    required this.boardSize,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Variations', style: text.headlineSmall),
          const SizedBox(height: 4),
          Text(
            'This move has ${siblings.length} branches. Tap one to jump in.',
            style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (var i = 0; i < siblings.length; i++)
                ActionChip(
                  avatar: MiniStone(
                    color: siblings[i].move?.player ?? StoneColor.black,
                    size: 16,
                  ),
                  label: Text(_label(siblings[i], i)),
                  onPressed: () => onSelect(siblings[i]),
                ),
            ],
          ),
        ],
      ),
    );
  }

  String _label(VariationNode node, int index) {
    final move = node.move;
    if (move == null) return 'Root';
    final tag = index == 0 ? 'Main' : 'Var $index';
    if (move.type != MoveType.placeStone || move.point == null) {
      return '$tag · pass';
    }
    return '$tag · ${_coord(move.point!, boardSize)}';
  }

  static String _coord(Point p, int size) {
    const skipI = 8;
    final col = p.col;
    final letter = String.fromCharCode(0x41 + (col < skipI ? col : col + 1));
    final row = size - p.row;
    return '$letter$row';
  }
}

Future<ReviewReport> _analyzeGame(
        ({GameState game, GameState? initial}) input) =>
    ReviewAnalyzer().analyze(input.game, initialState: input.initial);
