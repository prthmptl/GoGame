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
import '../../sgf/sgf_import.dart';
import '../board/board_canvas.dart';
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
      _index = state.history.length;
    });
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
                      const SizedBox(height: 2),
                      Text(
                        '${replayed.config.ruleset.label} rules · '
                        '${replayed.config.boardSize}×${replayed.config.boardSize} · '
                        'komi ${replayed.config.komi.toStringAsFixed(1)}',
                        style: text.labelSmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
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
        ],
      ),
    );
  }
}
