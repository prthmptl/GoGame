import 'package:flutter/material.dart';

import '../../data/lesson_repo.dart';
import '../../domain/lessons/course.dart';
import '../components/zen_components.dart';

class CoursesScreen extends StatefulWidget {
  final LessonRepo repo;
  final ValueChanged<Course> onOpen;
  const CoursesScreen({
    super.key,
    required this.repo,
    required this.onOpen,
  });

  @override
  State<CoursesScreen> createState() => _CoursesScreenState();
}

class _CoursesScreenState extends State<CoursesScreen> {
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
        title: const Text('Courses'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.maybePop(context),
        ),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            Text(
              'Structured courses. Each lesson is a few short sections ending in a real puzzle.',
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 12),
            for (final course in widget.repo.courses) ...[
              _CourseCard(
                course: course,
                completed: widget.repo.completedCount(course),
                onTap: () => widget.onOpen(course),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
    );
  }
}

class _CourseCard extends StatelessWidget {
  final Course course;
  final int completed;
  final VoidCallback onTap;
  const _CourseCard({
    required this.course,
    required this.completed,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final total = course.lessons.length;
    final pct = total == 0 ? 0.0 : completed / total;
    return ZenCard(
      onTap: onTap,
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(course.subtitle.toUpperCase(),
              style: text.labelSmall
                  ?.copyWith(color: scheme.onSurfaceVariant, letterSpacing: 1.2)),
          const SizedBox(height: 4),
          Text(course.title, style: text.headlineMedium),
          const SizedBox(height: 4),
          Text(course.description,
              style: text.bodyMedium
                  ?.copyWith(color: scheme.onSurfaceVariant)),
          const SizedBox(height: 10),
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: LinearProgressIndicator(
              value: pct,
              minHeight: 6,
              backgroundColor: scheme.surfaceContainerHigh,
              valueColor: AlwaysStoppedAnimation(scheme.primary),
            ),
          ),
          const SizedBox(height: 4),
          Text('$completed / $total lessons complete',
              style: text.labelSmall
                  ?.copyWith(color: scheme.onSurfaceVariant)),
        ],
      ),
    );
  }
}
