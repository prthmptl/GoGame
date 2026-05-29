import 'package:flutter/material.dart';

import '../../data/lesson_repo.dart';
import '../../domain/lessons/course.dart';
import '../components/zen_components.dart';

class CourseScreen extends StatefulWidget {
  final Course course;
  final LessonRepo repo;
  final void Function(Course, Lesson, int index) onOpenLesson;

  const CourseScreen({
    super.key,
    required this.course,
    required this.repo,
    required this.onOpenLesson,
  });

  @override
  State<CourseScreen> createState() => _CourseScreenState();
}

class _CourseScreenState extends State<CourseScreen> {
  @override
  void initState() {
    super.initState();
    widget.repo.addListener(_onChanged);
  }

  @override
  void dispose() {
    widget.repo.removeListener(_onChanged);
    super.dispose();
  }

  void _onChanged() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Scaffold(
      backgroundColor: scheme.surface,
      appBar: AppBar(
        title: Text(widget.course.title),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            Text(widget.course.description,
                style: text.bodyMedium
                    ?.copyWith(color: scheme.onSurfaceVariant)),
            const SizedBox(height: 16),
            for (var i = 0; i < widget.course.lessons.length; i++) ...[
              _LessonRow(
                course: widget.course,
                lesson: widget.course.lessons[i],
                index: i,
                done: widget.repo
                    .isLessonComplete(widget.course.id,
                        widget.course.lessons[i].id),
                onTap: () =>
                    widget.onOpenLesson(widget.course, widget.course.lessons[i], i),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
    );
  }
}

class _LessonRow extends StatelessWidget {
  final Course course;
  final Lesson lesson;
  final int index;
  final bool done;
  final VoidCallback onTap;
  const _LessonRow({
    required this.course,
    required this.lesson,
    required this.index,
    required this.done,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return ZenCard(
      onTap: onTap,
      container:
          done ? scheme.primaryContainer : scheme.surfaceContainerLow,
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: done
                  ? scheme.onPrimaryContainer.withValues(alpha: 0.15)
                  : scheme.surfaceContainerHigh,
            ),
            alignment: Alignment.center,
            child: done
                ? Icon(Icons.check, color: scheme.onPrimaryContainer)
                : Text('${index + 1}',
                    style: text.headlineSmall),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(lesson.title,
                    style: text.headlineSmall?.copyWith(
                      color: done
                          ? scheme.onPrimaryContainer
                          : scheme.onSurface,
                    )),
                Text(lesson.summary,
                    style: text.bodyMedium?.copyWith(
                      color: done
                          ? scheme.onPrimaryContainer.withValues(alpha: 0.85)
                          : scheme.onSurfaceVariant,
                    )),
              ],
            ),
          ),
          Icon(Icons.chevron_right,
              color: done
                  ? scheme.onPrimaryContainer
                  : scheme.onSurfaceVariant),
        ],
      ),
    );
  }
}
