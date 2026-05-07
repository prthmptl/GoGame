import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show ScrollDirection;
import 'package:flutter/services.dart';
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
              routes: [
                GoRoute(
                  path: 'lesson/:idx',
                  builder: (context, state) => LessonDetailScreen(
                    index: int.parse(state.pathParameters['idx']!),
                  ),
                ),
              ],
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
          builder: (context, state) =>
              _StandalonePage(child: RulesScreen(onBack: () => context.pop())),
        ),
        GoRoute(
          path: '/setup-local',
          builder: (context, state) => _StandalonePage(
            child: SetupScreen(
              isAi: false,
              onStart: (setup) {
                _gameVm.startGame(
                  config: setup.config,
                  opponent: setup.opponent,
                  aiPlays: setup.aiColor,
                  showHints: widget.settings.value.beginnerHints,
                );
                context.pushReplacement('/game');
              },
            ),
          ),
        ),
        GoRoute(
          path: '/setup-ai',
          builder: (context, state) => _StandalonePage(
            child: SetupScreen(
              isAi: true,
              onStart: (setup) {
                _gameVm.startGame(
                  config: setup.config,
                  opponent: setup.opponent,
                  aiPlays: setup.aiColor,
                  showHints: widget.settings.value.beginnerHints,
                );
                context.pushReplacement('/game');
              },
            ),
          ),
        ),
        GoRoute(
          path: '/game',
          builder: (context, state) => _StandalonePage(
            child: GameScreen(
              vm: _gameVm,
              settings: widget.settings,
              onExit: () => context.go('/play'),
            ),
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

class _StandalonePage extends StatelessWidget {
  final Widget child;
  const _StandalonePage({required this.child});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Theme.of(context).colorScheme.surface,
      body: SafeArea(child: child),
    );
  }
}

class _Chrome extends StatefulWidget {
  final Widget child;
  final String currentLocation;
  const _Chrome({required this.child, required this.currentLocation});

  static const _routes = ['/play', '/learn', '/review', '/settings'];

  @override
  State<_Chrome> createState() => _ChromeState();
}

class _ChromeState extends State<_Chrome> {
  static const _bottomBarHeight = 68.0;
  bool _chromeHidden = false;
  final List<String> _tabHistory = [];

  @override
  void initState() {
    super.initState();
    _tabHistory.add(_tabRootFor(widget.currentLocation));
  }

  @override
  void didUpdateWidget(covariant _Chrome oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentLocation != widget.currentLocation) {
      _setChromeHidden(false);
      final tab = _tabRootFor(widget.currentLocation);
      if (_tabHistory.isEmpty || _tabHistory.last != tab) {
        _tabHistory.remove(tab);
        _tabHistory.add(tab);
      }
    }
  }

  String _tabRootFor(String location) {
    return _Chrome._routes.firstWhere(
      (r) => location.startsWith(r),
      orElse: () => _Chrome._routes.first,
    );
  }

  Future<bool> _handleBack() async {
    if (context.canPop()) {
      context.pop();
      return false;
    }
    if (_tabHistory.length > 1) {
      _tabHistory.removeLast();
      final previous = _tabHistory.last;
      // Remove so didUpdateWidget re-adds it as the current tab.
      _tabHistory.removeLast();
      context.go(previous);
      return false;
    }
    if (_tabHistory.isNotEmpty &&
        _tabHistory.last != _Chrome._routes.first) {
      _tabHistory.clear();
      context.go(_Chrome._routes.first);
      return false;
    }
    return true;
  }

  void _setChromeHidden(bool hidden) {
    if (_chromeHidden == hidden) return;
    setState(() => _chromeHidden = hidden);
  }

  bool _handleScroll(ScrollNotification notification) {
    if (notification.metrics.axis != Axis.vertical) return false;
    if (notification.metrics.maxScrollExtent <= 16) {
      _setChromeHidden(false);
      return false;
    }
    if (notification.metrics.pixels <= 8) {
      _setChromeHidden(false);
      return false;
    }
    if (notification is ScrollUpdateNotification) {
      final delta = notification.scrollDelta;
      if (delta == null) return false;
      if (delta > 4 && notification.metrics.pixels > 24) {
        _setChromeHidden(true);
      } else if (delta < -4) {
        _setChromeHidden(false);
      }
      return false;
    }
    if (notification is UserScrollNotification) {
      if (notification.direction == ScrollDirection.reverse) {
        _setChromeHidden(true);
      } else if (notification.direction == ScrollDirection.forward) {
        _setChromeHidden(false);
      }
    }
    return false;
  }

  int _indexFor(String location) {
    final i = _Chrome._routes.indexWhere((r) => location.startsWith(r));
    return i < 0 ? 0 : i;
  }

  Widget _navTile(BuildContext context,
      {required IconData icon,
      required String label,
      required int index}) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final selected = _indexFor(widget.currentLocation) == index;
    final color = selected ? scheme.onSurface : scheme.onSurfaceVariant;
    return InkWell(
      onTap: () {
        if (!selected) HapticFeedback.selectionClick();
        context.go(_Chrome._routes[index]);
      },
      customBorder: const StadiumBorder(),
      splashColor: Colors.transparent,
      highlightColor: Colors.transparent,
      hoverColor: Colors.transparent,
      focusColor: Colors.transparent,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 220),
        curve: Curves.easeOutCubic,
        padding: EdgeInsets.symmetric(
          horizontal: selected ? 16 : 12,
          vertical: 10,
        ),
        decoration: BoxDecoration(
          color:
              selected ? scheme.surfaceContainerHigh : Colors.transparent,
          borderRadius: BorderRadius.circular(22),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 22, color: color),
            AnimatedSize(
              duration: const Duration(milliseconds: 220),
              curve: Curves.easeOutCubic,
              child: selected
                  ? Padding(
                      padding: const EdgeInsets.only(left: 8),
                      child: Text(
                        label,
                        style: text.labelMedium?.copyWith(
                          color: scheme.onSurface,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    )
                  : const SizedBox.shrink(),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final viewPadding = MediaQuery.viewPaddingOf(context);
    final topChromeHeight = viewPadding.top + kToolbarHeight;
    final bottomChromeHeight = viewPadding.bottom + _bottomBarHeight;
    final style = SystemUiOverlayStyle.dark.copyWith(
      statusBarColor: Colors.transparent,
      systemNavigationBarColor: scheme.surfaceContainer,
      systemNavigationBarIconBrightness: Brightness.dark,
    );

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: style,
      child: PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, _) async {
          if (didPop) return;
          final shouldExit = await _handleBack();
          if (shouldExit) {
            SystemNavigator.pop();
          }
        },
        child: Scaffold(
        backgroundColor: scheme.surface,
        body: NotificationListener<ScrollNotification>(
          onNotification: _handleScroll,
          child: Stack(
            children: [
              Positioned.fill(
                child: AnimatedPadding(
                  duration: const Duration(milliseconds: 180),
                  curve: Curves.easeOutCubic,
                  padding: EdgeInsets.only(
                    top: _chromeHidden ? viewPadding.top : topChromeHeight,
                    bottom:
                        _chromeHidden ? viewPadding.bottom : bottomChromeHeight,
                  ),
                  child: widget.child,
                ),
              ),
              Positioned(
                left: 0,
                right: 0,
                top: 0,
                child: AnimatedSlide(
                  duration: const Duration(milliseconds: 180),
                  curve: Curves.easeOutCubic,
                  offset: _chromeHidden ? const Offset(0, -1) : Offset.zero,
                  child: Material(
                    color: scheme.surface,
                    child: SafeArea(
                      bottom: false,
                      child: SizedBox(
                        height: kToolbarHeight,
                        child: Center(
                          child: Text(
                            'Weiqi',
                            style: text.headlineSmall
                                ?.copyWith(fontWeight: FontWeight.w600),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                child: AnimatedSlide(
                  duration: const Duration(milliseconds: 180),
                  curve: Curves.easeOutCubic,
                  offset: _chromeHidden ? const Offset(0, 1) : Offset.zero,
                  child: Material(
                    color: scheme.surfaceContainer,
                    child: SafeArea(
                      top: false,
                      child: SizedBox(
                        height: _bottomBarHeight,
                        child: Padding(
                          padding:
                              const EdgeInsets.symmetric(horizontal: 8),
                          child: Row(
                            mainAxisAlignment:
                                MainAxisAlignment.spaceEvenly,
                            children: [
                              _navTile(context,
                                  icon: Icons.grid_on_outlined,
                                  label: 'Play',
                                  index: 0),
                              _navTile(context,
                                  icon: Icons.menu_book_outlined,
                                  label: 'Learn',
                                  index: 1),
                              _navTile(context,
                                  icon: Icons.rate_review_outlined,
                                  label: 'Review',
                                  index: 2),
                              _navTile(context,
                                  icon: Icons.settings_outlined,
                                  label: 'Settings',
                                  index: 3),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
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
      onPlayLocal: () => context.push('/setup-local'),
      onPlayAi: () => context.push('/setup-ai'),
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
