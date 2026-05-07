import 'package:flutter/material.dart';

import '../../domain/board.dart';
import '../../domain/models.dart';
import '../board/board_canvas.dart';
import '../components/zen_components.dart';

class _Diagram {
  final int boardSize;
  final List<Point> black;
  final List<Point> white;
  final Set<Point> markers;
  final String caption;
  const _Diagram({
    required this.boardSize,
    required this.caption,
    this.black = const [],
    this.white = const [],
    this.markers = const {},
  });
}

class _Lesson {
  final String title;
  final String summary;
  final String body;
  final List<_Diagram> diagrams;
  const _Lesson({
    required this.title,
    required this.summary,
    required this.body,
    required this.diagrams,
  });
}

final _lessons = <_Lesson>[
  _Lesson(
    title: 'Liberties',
    summary: 'Every stone has empty neighbors keeping it alive.',
    body:
        "A stone's liberties are the empty intersections directly adjacent to it (up, down, left, right). "
        'Connected stones of the same color share liberties. '
        'When a group has zero liberties, it is captured and removed from the board. '
        'Edges and corners reduce the number of liberties — a corner stone starts with only two.',
    diagrams: [
      _Diagram(
        boardSize: 7,
        black: [const Point(3, 3)],
        markers: {
          const Point(2, 3),
          const Point(4, 3),
          const Point(3, 2),
          const Point(3, 4)
        },
        caption: 'Center stone has 4 liberties (red dots).',
      ),
      _Diagram(
        boardSize: 7,
        black: [const Point(0, 0)],
        markers: {const Point(1, 0), const Point(0, 1)},
        caption: 'Corner stone has only 2 liberties.',
      ),
    ],
  ),
  _Lesson(
    title: 'Capturing',
    summary: 'Surround your opponent to remove their stones.',
    body:
        'To capture, you must take away the last liberty of an opposing group. '
        'If your move would self-capture (suicide), it is illegal — unless the same move captures '
        'an opposing group first, restoring liberties to your own. '
        'Captured stones are returned to the bowl and the points become empty again.',
    diagrams: [
      _Diagram(
        boardSize: 7,
        white: [const Point(3, 3)],
        black: [const Point(2, 3), const Point(4, 3), const Point(3, 2)],
        markers: {const Point(3, 4)},
        caption: 'Black plays at the marked point — White is captured.',
      ),
    ],
  ),
  _Lesson(
    title: 'Eyes and Life',
    summary: 'Two true eyes make a group permanently alive.',
    body:
        'An eye is an empty point inside a group that the opponent cannot fill. '
        'A group with two separate eyes can never be captured: filling either eye would be suicide. '
        'Strong shapes form two eyes early; weak shapes are reduced to one eye and die. '
        'Beware false eyes — points that look like eyes but can be taken away tactically.',
    diagrams: [
      _Diagram(
        boardSize: 7,
        black: [
          const Point(0, 1),
          const Point(0, 2),
          const Point(0, 3),
          const Point(0, 4),
          const Point(1, 0),
          const Point(1, 4),
          const Point(2, 0),
          const Point(2, 4),
          const Point(3, 0),
          const Point(3, 1),
          const Point(3, 2),
          const Point(3, 3),
          const Point(3, 4),
        ],
        markers: {const Point(1, 2), const Point(2, 2)},
        caption:
            'Two eyes (red marks). White cannot fill either — alive forever.',
      ),
    ],
  ),
  _Lesson(
    title: 'Ko',
    summary: 'You cannot immediately recreate the previous position.',
    body:
        'After a one-stone capture, your opponent cannot recapture if doing so would return the board '
        'to the position just before their previous move. They must play elsewhere first (a ko threat). '
        'Positional superko extends this rule: no move may recreate any earlier whole-board position. '
        'Ko fights are about who has more meaningful threats elsewhere on the board.',
    diagrams: [
      _Diagram(
        boardSize: 7,
        black: [const Point(2, 3), const Point(3, 4), const Point(4, 3)],
        white: [
          const Point(2, 2),
          const Point(3, 1),
          const Point(4, 2),
          const Point(3, 3)
        ],
        markers: {const Point(3, 2)},
        caption: 'Marked point: a classic ko shape between Black and White.',
      ),
    ],
  ),
  const _Lesson(
    title: 'Scoring (Chinese)',
    summary: 'Stones on the board + surrounded territory + komi.',
    body:
        'After both players pass, the game enters scoring. Mark dead stones (groups that cannot make two eyes) '
        'for removal. Then count: each player\'s score is their stones still on the board, plus the empty '
        'points surrounded only by their color. White also receives komi (typically 7.5) to compensate '
        "Black's first-move advantage. Highest total wins.",
    diagrams: [
      _Diagram(
        boardSize: 7,
        black: [
          Point(0, 3),
          Point(1, 3),
          Point(2, 3),
          Point(3, 3),
          Point(3, 2),
          Point(3, 1),
          Point(3, 0),
        ],
        white: [
          Point(0, 4),
          Point(1, 4),
          Point(2, 4),
          Point(3, 4),
          Point(4, 3),
          Point(4, 4),
          Point(4, 5),
          Point(4, 6),
        ],
        caption:
            'Black surrounds the top-left; White surrounds the rest. Each empty region counts for the side that surrounds it.',
      ),
    ],
  ),
];

class TutorialScreen extends StatefulWidget {
  const TutorialScreen({super.key});

  @override
  State<TutorialScreen> createState() => _TutorialScreenState();
}

class _TutorialScreenState extends State<TutorialScreen> {
  _Lesson? _current;

  @override
  Widget build(BuildContext context) {
    final inLesson = _current != null;
    return PopScope(
      canPop: !inLesson,
      onPopInvokedWithResult: (didPop, _) {
        if (didPop) return;
        if (inLesson) setState(() => _current = null);
      },
      child: inLesson
          ? _Detail(
              lesson: _current!,
              onBack: () => setState(() => _current = null))
          : _List(onPick: (l) => setState(() => _current = l)),
    );
  }
}

class _List extends StatelessWidget {
  final ValueChanged<_Lesson> onPick;
  const _List({required this.onPick});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Learn',
              style:
                  text.labelMedium?.copyWith(color: scheme.onSurfaceVariant)),
          Text('Beginner Path',
              style:
                  text.headlineMedium?.copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          Text(
            'Five short lessons covering everything you need to play your first full game.',
            style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 16),
          ..._lessons.asMap().entries.map((entry) {
            final i = entry.key;
            final l = entry.value;
            return Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: ZenCard(
                onTap: () => onPick(l),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('Lesson ${i + 1}',
                              style: text.labelSmall
                                  ?.copyWith(color: scheme.onSurfaceVariant)),
                          Text(l.title, style: text.headlineSmall),
                          Text(l.summary,
                              style: text.bodyMedium
                                  ?.copyWith(color: scheme.onSurfaceVariant)),
                        ],
                      ),
                    ),
                    Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
                  ],
                ),
              ),
            );
          }),
        ],
      ),
    );
  }
}

class _Detail extends StatelessWidget {
  final _Lesson lesson;
  final VoidCallback onBack;
  const _Detail({required this.lesson, required this.onBack});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(children: [
            IconButton(onPressed: onBack, icon: const Icon(Icons.arrow_back)),
            Text('Beginner path',
                style:
                    text.labelMedium?.copyWith(color: scheme.onSurfaceVariant)),
          ]),
          Text(lesson.title, style: text.displayLarge),
          Text(lesson.summary,
              style: text.bodyLarge?.copyWith(color: scheme.onSurfaceVariant)),
          const SizedBox(height: 12),
          ZenCard(child: Text(lesson.body, style: text.bodyMedium)),
          const SizedBox(height: 12),
          ...lesson.diagrams.map((d) => Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: _DiagramCard(diagram: d),
              )),
          TextButton(onPressed: onBack, child: const Text('Back to lessons')),
        ],
      ),
    );
  }
}

class _DiagramCard extends StatelessWidget {
  final _Diagram diagram;
  const _DiagramCard({required this.diagram});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    var b = Board.empty(diagram.boardSize);
    for (final p in diagram.black) {
      b = b.setCell(p, CellState.black);
    }
    for (final p in diagram.white) {
      b = b.setCell(p, CellState.white);
    }
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AspectRatio(
            aspectRatio: 1,
            child: BoardCanvas(
              board: b,
              overlay: BoardOverlay(markers: diagram.markers),
            ),
          ),
          const SizedBox(height: 8),
          Text(diagram.caption,
              style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant)),
        ],
      ),
    );
  }
}
