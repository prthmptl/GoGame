import 'dart:io';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';

import 'data/saved_game.dart';
import 'data/saved_game_repo.dart';
import 'data/settings_store.dart';
import 'domain/models.dart';
import 'ui/screens/game_screen.dart';
import 'ui/screens/game_view_model.dart';
import 'ui/screens/home_screen.dart';
import 'ui/screens/review_screen.dart';
import 'ui/screens/rules_screen.dart';
import 'ui/screens/settings_screen.dart';
import 'ui/screens/setup_screen.dart';
import 'ui/screens/tutorial_screen.dart';
import 'ui/theme.dart';

class WeiqiApp extends StatefulWidget {
  final SavedGameRepo repo;
  final SettingsStore settings;
  const WeiqiApp({super.key, required this.repo, required this.settings});

  @override
  State<WeiqiApp> createState() => _WeiqiAppState();
}

class _WeiqiAppState extends State<WeiqiApp> {
  late final GameViewModel _gameVm = GameViewModel(repo: widget.repo);
  late final GoRouter _router = _buildRouter();

  GoRouter _buildRouter() {
    return GoRouter(
      initialLocation: '/play',
      routes: [
        ShellRoute(
          builder: (context, state, child) =>
              _Chrome(currentLocation: state.matchedLocation, child: child),
          routes: [
            GoRoute(
              path: '/play',
              builder: (context, state) =>
                  _PlayTab(repo: widget.repo, vm: _gameVm),
            ),
            GoRoute(
              path: '/learn',
              builder: (context, state) => const TutorialScreen(),
            ),
            GoRoute(
              path: '/review',
              builder: (context, state) => ReviewScreen(
                savedGameId: state.uri.queryParameters['id'],
                repo: widget.repo,
                settings: widget.settings,
              ),
            ),
            GoRoute(
              path: '/settings',
              builder: (context, state) =>
                  SettingsScreen(store: widget.settings),
            ),
          ],
        ),
        GoRoute(
          path: '/rules',
          builder: (context, state) => RulesScreen(onBack: () => context.pop()),
        ),
        GoRoute(
          path: '/setup-local',
          builder: (context, state) => SetupScreen(
            isAi: false,
            onStart: (setup) {
              _gameVm.startGame(
                config: setup.config,
                opponent: setup.opponent,
                aiPlays: setup.aiColor,
                showHints: widget.settings.value.beginnerHints,
              );
              context.go('/game');
            },
          ),
        ),
        GoRoute(
          path: '/setup-ai',
          builder: (context, state) => SetupScreen(
            isAi: true,
            onStart: (setup) {
              _gameVm.startGame(
                config: setup.config,
                opponent: setup.opponent,
                aiPlays: setup.aiColor,
                showHints: widget.settings.value.beginnerHints,
              );
              context.go('/game');
            },
          ),
        ),
        GoRoute(
          path: '/game',
          builder: (context, state) => GameScreen(
            vm: _gameVm,
            settings: widget.settings,
            onExit: () => context.go('/play'),
          ),
        ),
      ],
    );
  }

  @override
  void dispose() {
    _gameVm.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      title: 'Weiqi',
      debugShowCheckedModeBanner: false,
      theme: buildWeiqiTheme(),
      routerConfig: _router,
    );
  }
}

class _Chrome extends StatelessWidget {
  final Widget child;
  final String currentLocation;
  const _Chrome({required this.child, required this.currentLocation});

  static const _routes = ['/play', '/learn', '/review', '/settings'];

  int _indexFor(String location) {
    final i = _routes.indexWhere((r) => location.startsWith(r));
    return i < 0 ? 0 : i;
  }

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return Scaffold(
      appBar: AppBar(
        title: Text('Weiqi',
            style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w600)),
      ),
      body: SafeArea(child: child),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _indexFor(currentLocation),
        onDestinationSelected: (i) => context.go(_routes[i]),
        destinations: const [
          NavigationDestination(
              icon: Icon(Icons.grid_on_outlined), label: 'Play'),
          NavigationDestination(
              icon: Icon(Icons.menu_book_outlined), label: 'Learn'),
          NavigationDestination(
              icon: Icon(Icons.rate_review_outlined), label: 'Review'),
          NavigationDestination(
              icon: Icon(Icons.settings_outlined), label: 'Settings'),
        ],
      ),
    );
  }
}

class _PlayTab extends StatefulWidget {
  final SavedGameRepo repo;
  final GameViewModel vm;
  const _PlayTab({required this.repo, required this.vm});

  @override
  State<_PlayTab> createState() => _PlayTabState();
}

class _PlayTabState extends State<_PlayTab> {
  bool _hasSaved = false;
  List<RecentGame> _recents = const [];

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    final hasSaved = await widget.repo.loadCurrent() != null;
    final completed = await widget.repo.listCompleted(limit: 5);
    if (!mounted) return;
    setState(() {
      _hasSaved = hasSaved;
      _recents = completed.map(_toRecent).toList(growable: false);
    });
  }

  RecentGame _toRecent(SavedGameEntity e) {
    StoneColor you;
    try {
      you = StoneColor.values
          .firstWhere((s) => s.name.toUpperCase() == e.youColor);
    } catch (_) {
      you = StoneColor.black;
    }
    final opponentName =
        e.opponentLabel.startsWith('AI') ? e.opponentLabel : 'Local';
    final result = e.resultLabel.isEmpty ? '—' : e.resultLabel;
    return RecentGame(
      id: e.id,
      opponent: opponentName,
      result: result,
      boardSize: e.boardSize,
      date: _relativeDate(e.updatedAtMillis),
      youPlayed: you,
      sgfPath: e.sgfPath.isEmpty ? null : e.sgfPath,
    );
  }

  String _relativeDate(int millis) {
    final now = DateTime.now().millisecondsSinceEpoch;
    final diffSec = (now - millis) ~/ 1000;
    if (diffSec < 60) return 'Just now';
    if (diffSec < 3600) return '${diffSec ~/ 60} min ago';
    if (diffSec < 86400) return '${diffSec ~/ 3600}h ago';
    return '${diffSec ~/ 86400}d ago';
  }

  Future<void> _shareRecent(RecentGame game) async {
    final path = game.sgfPath;
    if (path == null) return;
    final f = File(path);
    if (!f.existsSync()) return;
    await Share.shareXFiles([XFile(f.path, mimeType: 'application/x-go-sgf')],
        subject: 'Weiqi game (SGF)');
  }

  Future<void> _deleteRecent(RecentGame game) async {
    await widget.repo.delete(game.id);
    await _refresh();
  }

  @override
  Widget build(BuildContext context) {
    return HomeScreen(
      onPlayLocal: () => context.go('/setup-local'),
      onPlayAi: () => context.go('/setup-ai'),
      onRules: () => context.push('/rules'),
      onResume: _hasSaved
          ? () async {
              final ok = await widget.vm.resumeCurrent();
              if (!ok || !context.mounted) return;
              context.go('/game');
            }
          : null,
      recents: _recents,
      onOpenRecent: (game) {
        if (game.sgfPath != null) context.go('/review?id=${game.id}');
      },
      onShareRecent: _shareRecent,
      onDeleteRecent: _deleteRecent,
    );
  }
}
