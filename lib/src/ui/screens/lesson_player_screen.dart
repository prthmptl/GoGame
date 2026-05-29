import 'package:flutter/material.dart';

import '../../data/lesson_repo.dart';
import '../../data/puzzle_repo.dart';
import '../../data/settings_store.dart';
import '../../domain/board.dart';
import '../../domain/lessons/course.dart';
import '../../domain/models.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';
import 'puzzle_screen.dart';

class LessonPlayerScreen extends StatefulWidget {
  final Course course;
  final Lesson lesson;
  final LessonRepo lessons;
  final PuzzleRepo puzzles;
  final SettingsStore settings;

  const LessonPlayerScreen({
    super.key,
    required this.course,
    required this.lesson,
    required this.lessons,
    required this.puzzles,
    required this.settings,
  });

  @override
  State<LessonPlayerScreen> createState() => _LessonPlayerScreenState();
}

class _LessonPlayerScreenState extends State<LessonPlayerScreen> {
  int _index = 0;

  void _next() {
    if (_index < widget.lesson.sections.length - 1) {
      setState(() => _index++);
    } else {
      widget.lessons.markComplete(widget.course.id, widget.lesson.id);
      Navigator.maybePop(context);
    }
  }

  void _prev() {
    if (_index > 0) setState(() => _index--);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final section = widget.lesson.sections[_index];
    final isLast = _index == widget.lesson.sections.length - 1;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: Text(widget.lesson.title),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: LinearProgressIndicator(
                  value: (_index + 1) / widget.lesson.sections.length,
                  minHeight: 4,
                  backgroundColor: scheme.surfaceContainerHigh,
                  valueColor: AlwaysStoppedAnimation(scheme.primary),
                ),
              ),
            ),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [_buildSection(section, scheme, text)],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: _index > 0 ? _prev : null,
                      child: const Text('BACK'),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    flex: 2,
                    child: FilledButton(
                      onPressed: _next,
                      child: Text(isLast ? 'FINISH' : 'NEXT'),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSection(
      LessonSection section, ColorScheme scheme, TextTheme text) {
    if (section is TextSection) {
      return Text(section.body,
          style: text.bodyLarge?.copyWith(height: 1.5));
    }
    if (section is DiagramSection) {
      var board = Board.empty(section.boardSize);
      board = board.setMany([
        for (final p in section.black) MapEntry(p, CellState.black),
        for (final p in section.white) MapEntry(p, CellState.white),
      ]);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AspectRatio(
            aspectRatio: 1,
            child: ZenCard(
              contentPadding: const EdgeInsets.all(6),
              child: BoardCanvas(
                board: board,
                overlay: BoardOverlay(
                  markers: section.markers.toSet(),
                ),
                appearance: BoardAppearance.classicWood,
              ),
            ),
          ),
          if (section.caption != null) ...[
            const SizedBox(height: 8),
            Text(section.caption!,
                style: text.bodyMedium
                    ?.copyWith(color: scheme.onSurfaceVariant)),
          ],
        ],
      );
    }
    if (section is ExerciseSection) {
      final puzzle = widget.puzzles.puzzleById(section.puzzleId);
      if (puzzle == null) {
        return Text('Exercise unavailable.',
            style: text.bodyMedium?.copyWith(color: scheme.error));
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(section.prompt ?? puzzle.description, style: text.bodyLarge),
          const SizedBox(height: 12),
          SizedBox(
            height: 56,
            child: FilledButton.icon(
              onPressed: () {
                Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => PuzzleScreen(
                    puzzle: puzzle,
                    repo: widget.puzzles,
                    settings: widget.settings,
                  ),
                ));
              },
              icon: const Icon(Icons.play_arrow),
              label: const Text('OPEN EXERCISE'),
            ),
          ),
        ],
      );
    }
    return const SizedBox.shrink();
  }
}
