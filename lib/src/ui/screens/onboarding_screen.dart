import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../data/profile_store.dart';
import '../../data/puzzle_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/bots/bot_catalog.dart';
import '../../domain/bots/bot_profile.dart';
import '../../domain/models.dart';
import '../../domain/puzzles/puzzle_session.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

const _calibrationPuzzleIds = ['p001', 'p004', 'p005'];

class OnboardingScreen extends StatefulWidget {
  final ProfileStore profile;
  final PuzzleRepo puzzles;
  final SettingsStore settings;
  final VoidCallback onFinish;
  final void Function(BotProfile bot) onPlayBot;

  const OnboardingScreen({
    super.key,
    required this.profile,
    required this.puzzles,
    required this.settings,
    required this.onFinish,
    required this.onPlayBot,
  });

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  int _step = 0;
  final _nameController = TextEditingController();
  int _calibIndex = 0;
  int _calibCorrect = 0;
  PuzzleSession? _calibSession;
  String? _calibFeedback;

  @override
  void initState() {
    super.initState();
    _nameController.text = widget.profile.value.name == 'Player'
        ? ''
        : widget.profile.value.name;
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  void _startCalibration() {
    setState(() {
      _step = 2;
      _calibIndex = 0;
      _calibCorrect = 0;
      _loadCalibPuzzle();
    });
  }

  void _loadCalibPuzzle() {
    if (_calibIndex >= _calibrationPuzzleIds.length) {
      _finishCalibration();
      return;
    }
    final id = _calibrationPuzzleIds[_calibIndex];
    final puzzle = widget.puzzles.puzzleById(id);
    if (puzzle == null) {
      _calibIndex++;
      _loadCalibPuzzle();
      return;
    }
    _calibSession = PuzzleSession.start(puzzle);
    _calibFeedback = null;
  }

  void _onCalibTap(Point point) {
    final session = _calibSession;
    if (session == null) return;
    if (session.status != PuzzleStatus.inProgress) return;
    final ok = session.play(point);
    if (!ok) {
      HapticFeedback.heavyImpact();
      setState(() => _calibFeedback = 'Not quite — let’s try the next.');
      Future.delayed(const Duration(milliseconds: 700), () {
        if (!mounted) return;
        setState(() {
          _calibIndex++;
          _loadCalibPuzzle();
        });
      });
      return;
    }
    HapticFeedback.lightImpact();
    if (session.status == PuzzleStatus.solved) {
      setState(() {
        _calibCorrect++;
        _calibFeedback = 'Solved!';
      });
      Future.delayed(const Duration(milliseconds: 600), () {
        if (!mounted) return;
        setState(() {
          _calibIndex++;
          _loadCalibPuzzle();
        });
      });
    } else {
      setState(() {});
    }
  }

  Future<void> _finishCalibration() async {
    final rating = 600 + _calibCorrect * 200;
    await widget.profile.update((p) => p.copyWith(rating: rating));
    setState(() => _step = 3);
  }

  Future<void> _completeOnboarding(BotProfile? selectedBot) async {
    final name = _nameController.text.trim();
    await widget.profile.update((p) => p.copyWith(
          name: name.isEmpty ? 'Player' : name,
          onboarded: true,
        ));
    if (selectedBot != null) {
      widget.onPlayBot(selectedBot);
    } else {
      widget.onFinish();
    }
  }

  BotProfile _suggestedBot() {
    final rating = widget.profile.value.rating;
    BotProfile best = BotCatalog.all.first;
    var bestDistance = (best.rating - rating).abs();
    for (final b in BotCatalog.all) {
      final d = (b.rating - rating).abs();
      if (d < bestDistance) {
        best = b;
        bestDistance = d;
      }
    }
    return best;
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
    return Scaffold(
      backgroundColor: scheme.surface,
      body: SafeArea(
        child: switch (_step) {
          0 => _welcome(),
          1 => _nameStep(),
          2 => _calibrationStep(),
          _ => _resultStep(),
        },
      ),
    );
  }

  Widget _welcome() {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: 32),
          Text('Welcome to Go.', style: text.displayLarge),
          const SizedBox(height: 16),
          Text(
            'Two players place stones to surround territory. The simplest rules in games, the deepest play. We’ll take you through a quick warm-up.',
            style:
                text.bodyLarge?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const Spacer(),
          SizedBox(
            width: double.infinity,
            height: 56,
            child: FilledButton(
              onPressed: () => setState(() => _step = 1),
              child: const Text('GET STARTED'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _nameStep() {
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: 32),
          Text('What should we call you?', style: text.displayLarge),
          const SizedBox(height: 24),
          TextField(
            controller: _nameController,
            autofocus: true,
            decoration: const InputDecoration(
              labelText: 'Display name',
              border: OutlineInputBorder(),
            ),
          ),
          const Spacer(),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => setState(() => _step = 0),
                  child: const Text('BACK'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                flex: 2,
                child: FilledButton(
                  onPressed: _startCalibration,
                  child: const Text('CALIBRATE'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _calibrationStep() {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final session = _calibSession;
    if (session == null) {
      return const Center(child: CircularProgressIndicator());
    }
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
              'Calibration ${_calibIndex + 1} / ${_calibrationPuzzleIds.length}',
              style: text.labelMedium
                  ?.copyWith(color: scheme.onSurfaceVariant)),
          const SizedBox(height: 4),
          Text(session.puzzle.title, style: text.headlineMedium),
          const SizedBox(height: 4),
          Text(session.puzzle.description, style: text.bodyLarge),
          const SizedBox(height: 16),
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
                onTap: _onCalibTap,
              ),
            ),
          ),
          const SizedBox(height: 12),
          if (_calibFeedback != null)
            ZenCard(
              container: scheme.surfaceContainerHigh,
              child: Text(_calibFeedback!, style: text.bodyMedium),
            ),
        ],
      ),
    );
  }

  Widget _resultStep() {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final suggested = _suggestedBot();
    final rating = widget.profile.value.rating;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: 16),
          Text('Nice.', style: text.displayLarge),
          const SizedBox(height: 8),
          Text(
            'You solved $_calibCorrect of ${_calibrationPuzzleIds.length}. We’ll start you around $rating.',
            style: text.bodyLarge?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 24),
          ZenCard(
            container: scheme.primaryContainer,
            child: Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    color: scheme.onPrimaryContainer.withValues(alpha: 0.15),
                    shape: BoxShape.circle,
                  ),
                  alignment: Alignment.center,
                  child: Text(suggested.initials,
                      style: text.headlineSmall?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: scheme.onPrimaryContainer,
                      )),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('SUGGESTED OPPONENT',
                          style: text.labelSmall?.copyWith(
                            color: scheme.onPrimaryContainer
                                .withValues(alpha: 0.85),
                            letterSpacing: 1.2,
                          )),
                      Text(
                        '${suggested.name} ${suggested.countryEmoji}',
                        style: text.headlineSmall?.copyWith(
                            color: scheme.onPrimaryContainer),
                      ),
                      Text(
                        '${suggested.rankLabel} · ${suggested.style.label}',
                        style: text.bodyMedium?.copyWith(
                            color: scheme.onPrimaryContainer
                                .withValues(alpha: 0.85)),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const Spacer(),
          SizedBox(
            width: double.infinity,
            height: 56,
            child: FilledButton.icon(
              onPressed: () => _completeOnboarding(suggested),
              icon: const Icon(Icons.play_arrow),
              label: Text('PLAY ${suggested.name.toUpperCase()}'),
            ),
          ),
          const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            height: 48,
            child: OutlinedButton(
              onPressed: () => _completeOnboarding(null),
              child: const Text('FINISH'),
            ),
          ),
        ],
      ),
    );
  }
}
