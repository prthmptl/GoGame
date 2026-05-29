import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../data/puzzle_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/models.dart';
import '../../domain/puzzles/puzzle.dart';
import '../../domain/puzzles/puzzle_session.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

const _kRushSeconds = 180;
const _kMaxStrikes = 3;

class PuzzleRushScreen extends StatefulWidget {
  final PuzzleRepo repo;
  final SettingsStore settings;
  const PuzzleRushScreen({
    super.key,
    required this.repo,
    required this.settings,
  });

  @override
  State<PuzzleRushScreen> createState() => _PuzzleRushScreenState();
}

class _PuzzleRushScreenState extends State<PuzzleRushScreen> {
  late final List<Puzzle> _queue;
  int _queueIndex = 0;
  int _solved = 0;
  int _strikes = 0;
  int _remainingSeconds = _kRushSeconds;
  Timer? _timer;
  bool _running = true;
  PuzzleSession? _session;

  @override
  void initState() {
    super.initState();
    _queue = _shufflePool();
    _advanceSession();
    _timer = Timer.periodic(const Duration(seconds: 1), _tick);
  }

  List<Puzzle> _shufflePool() {
    final pool = [...widget.repo.puzzles];
    pool.sort((a, b) => a.difficulty.compareTo(b.difficulty));
    // Add some randomness so consecutive rushes feel different.
    final rng = math.Random();
    for (var i = pool.length - 1; i > 0; i--) {
      // Light shuffle within the same difficulty band so easier puzzles
      // still come first.
      final j = math.max(0, i - rng.nextInt(2));
      final t = pool[i];
      pool[i] = pool[j];
      pool[j] = t;
    }
    return pool;
  }

  void _tick(Timer _) {
    if (!_running) return;
    setState(() {
      _remainingSeconds = math.max(0, _remainingSeconds - 1);
      if (_remainingSeconds <= 0) _end();
    });
  }

  void _advanceSession() {
    if (_queue.isEmpty) {
      _end();
      return;
    }
    if (_queueIndex >= _queue.length) _queueIndex = 0;
    _session = PuzzleSession.start(_queue[_queueIndex]);
  }

  void _onTap(Point point) {
    final session = _session;
    if (session == null || !_running) return;
    final ok = session.play(point);
    if (!ok) {
      HapticFeedback.heavyImpact();
      setState(() => _strikes++);
      if (_strikes >= _kMaxStrikes) {
        _end();
      }
      return;
    }
    HapticFeedback.lightImpact();
    if (session.status == PuzzleStatus.solved) {
      setState(() {
        _solved++;
        _queueIndex++;
        _advanceSession();
      });
    } else if (session.status == PuzzleStatus.failed) {
      setState(() {
        _strikes++;
        if (_strikes >= _kMaxStrikes) {
          _end();
          return;
        }
        _queueIndex++;
        _advanceSession();
      });
    } else {
      setState(() {});
    }
  }

  void _end() {
    _running = false;
    _timer?.cancel();
    widget.repo.recordRushScore(_solved);
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
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
    final session = _session;
    if (!_running || session == null) {
      return _RushResults(
        solved: _solved,
        best: widget.repo.rushBest(),
        onExit: () => Navigator.maybePop(context),
      );
    }
    final mins = _remainingSeconds ~/ 60;
    final secs = _remainingSeconds % 60;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Puzzle Rush'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () {
            _end();
            Navigator.maybePop(context);
          },
        ),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 16),
            child: Center(
              child: Text(
                '${mins.toString().padLeft(2, '0')}:${secs.toString().padLeft(2, '0')}',
                style: text.headlineSmall?.copyWith(
                  color: _remainingSeconds <= 15 ? scheme.error : null,
                ),
              ),
            ),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            ZenCard(
              container: scheme.surfaceContainerLow,
              child: Row(
                children: [
                  _Stat(label: 'Solved', value: '$_solved'),
                  const SizedBox(width: 16),
                  _Stat(
                      label: 'Strikes',
                      value: '$_strikes/$_kMaxStrikes'),
                  const SizedBox(width: 16),
                  _Stat(
                      label: 'Best',
                      value: '${widget.repo.rushBest()}'),
                ],
              ),
            ),
            const SizedBox(height: 12),
            ZenCard(
              container: scheme.surfaceContainerLow,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(session.puzzle.title, style: text.headlineSmall),
                  const SizedBox(height: 2),
                  Text(session.puzzle.description, style: text.bodyMedium),
                ],
              ),
            ),
            const SizedBox(height: 12),
            AspectRatio(
              aspectRatio: 1,
              child: ZenCard(
                contentPadding: const EdgeInsets.all(6),
                child: BoardCanvas(
                  board: session.state.board,
                  overlay: BoardOverlay(
                    lastMove: session.state.lastMove?.point,
                  ),
                  appearance: _appearance(),
                  onTap: _onTap,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _RushResults extends StatelessWidget {
  final int solved;
  final int best;
  final VoidCallback onExit;
  const _RushResults({
    required this.solved,
    required this.best,
    required this.onExit,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final isNewBest = solved >= best && solved > 0;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: const Text('Rush over'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: onExit,
        ),
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              const SizedBox(height: 24),
              Text(isNewBest ? 'New personal best!' : 'Run complete',
                  style: text.displayLarge),
              const SizedBox(height: 24),
              Text('$solved',
                  style: text.displayLarge?.copyWith(
                    fontSize: 80,
                    color: scheme.primary,
                  )),
              Text('puzzles solved',
                  style: text.bodyLarge
                      ?.copyWith(color: scheme.onSurfaceVariant)),
              const SizedBox(height: 32),
              Text('Personal best: $best',
                  style: text.bodyMedium
                      ?.copyWith(color: scheme.onSurfaceVariant)),
              const Spacer(),
              SizedBox(
                width: double.infinity,
                height: 56,
                child: FilledButton.icon(
                  onPressed: onExit,
                  icon: const Icon(Icons.check),
                  label: const Text('DONE'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  final String label;
  final String value;
  const _Stat({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label.toUpperCase(),
            style: text.labelSmall?.copyWith(
                color: scheme.onSurfaceVariant, letterSpacing: 1.2)),
        Text(value, style: text.headlineSmall),
      ],
    );
  }
}
