package com.weiqi.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsTopHeight
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.GridOn
import androidx.compose.material.icons.automirrored.outlined.MenuBook
import androidx.compose.material.icons.outlined.RateReview
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.core.content.FileProvider
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.weiqi.data.SavedGameEntity
import com.weiqi.data.SavedGameRepo
import com.weiqi.data.SettingsStore
import com.weiqi.data.WeiqiDatabase
import com.weiqi.engine.StoneColor
import com.weiqi.ui.screens.GameScreen
import com.weiqi.ui.screens.GameViewModel
import com.weiqi.ui.screens.HomeScreen
import com.weiqi.ui.screens.RecentGame
import com.weiqi.ui.screens.ReviewScreen
import com.weiqi.ui.screens.RulesScreen
import com.weiqi.ui.screens.SettingsScreen
import com.weiqi.ui.screens.SetupScreen
import com.weiqi.ui.screens.TutorialScreen
import com.weiqi.ui.theme.WeiqiTheme
import kotlinx.coroutines.launch
import java.io.File

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { WeiqiTheme { AppNav() } }
    }
}

private data class TopTab(val route: String, val label: String, val icon: @Composable () -> Unit)

private fun routeMatchesTab(route: String?, tab: TopTab): Boolean =
    route == tab.route || route?.startsWith("${tab.route}?") == true

private val topTabs = listOf(
    TopTab("play", "Play") { Icon(Icons.Outlined.GridOn, contentDescription = "Play") },
    TopTab("learn", "Learn") { Icon(Icons.AutoMirrored.Outlined.MenuBook, contentDescription = "Learn") },
    TopTab("review", "Review") { Icon(Icons.Outlined.RateReview, contentDescription = "Review") },
    TopTab("settings", "Settings") { Icon(Icons.Outlined.Settings, contentDescription = "Settings") }
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AppNav() {
    val nav = rememberNavController()
    val ctx = LocalContext.current
    val repo = remember(ctx) {
        SavedGameRepo(WeiqiDatabase.get(ctx).savedGames(), ctx.applicationContext)
    }
    val settings by SettingsStore.get(ctx).state.collectAsState()
    val gameVm: GameViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : ViewModel> create(modelClass: Class<T>): T =
                GameViewModel(repo) as T
        }
    )
    val backStack by nav.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination?.route

    val showChrome = currentRoute == null || topTabs.any { tab -> routeMatchesTab(currentRoute, tab) }

    val scrollBehavior = TopAppBarDefaults.enterAlwaysScrollBehavior()
    val chromeHideOffset = with(LocalDensity.current) { 64.dp.toPx() }
    // Use the scroll state as a shared hide trigger; the bars animate their own slots.
    SideEffect {
        scrollBehavior.state.heightOffsetLimit = -chromeHideOffset
    }
    val limit = scrollBehavior.state.heightOffsetLimit
    val barsVisible = limit == 0f || scrollBehavior.state.heightOffset > limit / 2f
    val scope = rememberCoroutineScope()

    Scaffold(
        modifier = if (showChrome) Modifier.nestedScroll(scrollBehavior.nestedScrollConnection) else Modifier,
        topBar = {
            if (showChrome) {
                Column {
                    Spacer(Modifier.windowInsetsTopHeight(WindowInsets.statusBars))
                    AnimatedVisibility(
                        visible = barsVisible,
                        enter = slideInVertically(
                            animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing)
                        ) { -it } + expandVertically(
                            animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing),
                            expandFrom = Alignment.Top,
                            clip = false
                        ),
                        exit = slideOutVertically(
                            animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing)
                        ) { -it } + shrinkVertically(
                            animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing),
                            shrinkTowards = Alignment.Top,
                            clip = false
                        )
                    ) {
                        CenterAlignedTopAppBar(
                            title = {
                                Text(
                                    "Weiqi",
                                    style = MaterialTheme.typography.headlineSmall,
                                    fontWeight = FontWeight.SemiBold
                                )
                            },
                            colors = TopAppBarDefaults.centerAlignedTopAppBarColors(
                                containerColor = MaterialTheme.colorScheme.background
                            ),
                            windowInsets = WindowInsets(0.dp)
                        )
                    }
                }
            }
        },
        bottomBar = {
            if (showChrome) {
                AnimatedVisibility(
                    visible = barsVisible,
                    enter = slideInVertically(
                        animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing)
                    ) { it } + expandVertically(
                        animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing),
                        expandFrom = Alignment.Bottom,
                        clip = false
                    ),
                    exit = slideOutVertically(
                        animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing)
                    ) { it } + shrinkVertically(
                        animationSpec = tween(durationMillis = 220, easing = FastOutSlowInEasing),
                        shrinkTowards = Alignment.Bottom,
                        clip = false
                    )
                ) {
                    NavigationBar(containerColor = MaterialTheme.colorScheme.surfaceContainer) {
                        topTabs.forEach { tab ->
                            val selected = backStack?.destination?.hierarchy?.any { entry ->
                                routeMatchesTab(entry.route, tab)
                            } == true
                            NavigationBarItem(
                                selected = selected,
                                onClick = {
                                    nav.navigate(tab.route) {
                                        popUpTo(nav.graph.findStartDestination().id) { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                icon = tab.icon,
                                label = { Text(tab.label, style = MaterialTheme.typography.labelSmall) }
                            )
                        }
                    }
                }
            }
        }
    ) { padding ->
        NavHost(
            navController = nav,
            startDestination = "play",
            modifier = Modifier.padding(padding)
        ) {
            composable("play") {
                var hasSaved by remember { mutableStateOf(false) }
                var recentsVersion by remember { mutableStateOf(0) }
                val recents by produceState(initialValue = emptyList<RecentGame>(), repo, currentRoute, recentsVersion) {
                    hasSaved = repo.loadCurrent() != null
                    value = repo.listCompleted(limit = 5).map { it.toRecent() }
                }
                HomeScreen(
                    onPlayLocal = { nav.navigate("setup/local") },
                    onPlayAi = { nav.navigate("setup/ai") },
                    onRules = { nav.navigate("rules") },
                    onResume = if (hasSaved) {
                        {
                            scope.launch {
                                if (gameVm.resumeCurrent()) nav.navigate("game")
                            }
                        }
                    } else null,
                    recents = recents,
                    onOpenRecent = { game ->
                        if (game.sgfPath != null) {
                            nav.navigate("review?id=${game.id}")
                        }
                    },
                    onShareRecent = { game ->
                        val path = game.sgfPath ?: return@HomeScreen
                        shareSgf(ctx, File(path))
                    },
                    onDeleteRecent = { game ->
                        scope.launch {
                            repo.delete(game.id)
                            recentsVersion++
                        }
                    }
                )
            }
            composable("learn") { TutorialScreen() }
            composable(
                route = "review?id={id}",
                arguments = listOf(navArgument("id") { type = NavType.StringType; nullable = true; defaultValue = null })
            ) { entry ->
                val id = entry.arguments?.getString("id")
                ReviewScreen(savedGameId = id, repo = repo)
            }
            composable("settings") { SettingsScreen() }
            composable("rules") { RulesScreen(onBack = { nav.popBackStack() }) }
            composable("setup/local") {
                SetupScreen(isAi = false) { setup ->
                    gameVm.startGame(
                        config = setup.config,
                        opponent = setup.opponent,
                        aiPlays = setup.aiColor,
                        showHints = settings.beginnerHints
                    )
                    nav.navigate("game")
                }
            }
            composable("setup/ai") {
                SetupScreen(isAi = true) { setup ->
                    gameVm.startGame(
                        config = setup.config,
                        opponent = setup.opponent,
                        aiPlays = setup.aiColor,
                        showHints = settings.beginnerHints
                    )
                    nav.navigate("game")
                }
            }
            composable("game") {
                GameScreen(vm = gameVm, onExit = { nav.popBackStack("play", inclusive = false) })
            }
        }
    }
}

private fun shareSgf(ctx: android.content.Context, file: File) {
    if (!file.exists()) return
    val authority = "${ctx.packageName}.fileprovider"
    val uri = FileProvider.getUriForFile(ctx, authority, file)
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "application/x-go-sgf"
        putExtra(Intent.EXTRA_STREAM, uri)
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    ctx.startActivity(Intent.createChooser(send, "Share SGF").apply {
        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    })
}

private fun SavedGameEntity.toRecent(): RecentGame {
    val you = runCatching { StoneColor.valueOf(youColor) }.getOrDefault(StoneColor.BLACK)
    val opponentName = when {
        opponentLabel.startsWith("AI") -> opponentLabel
        else -> "Local"
    }
    val result = if (resultLabel.isBlank()) "—" else resultLabel
    return RecentGame(
        id = id,
        opponent = opponentName,
        result = result,
        boardSize = boardSize,
        date = relativeDate(updatedAtMillis),
        youPlayed = you,
        sgfPath = sgfPath.takeIf { it.isNotBlank() }
    )
}

private fun relativeDate(millis: Long): String {
    val now = System.currentTimeMillis()
    val diffSec = (now - millis) / 1000
    return when {
        diffSec < 60 -> "Just now"
        diffSec < 3600 -> "${diffSec / 60} min ago"
        diffSec < 86_400 -> "${diffSec / 3600}h ago"
        diffSec < 7 * 86_400 -> "${diffSec / 86_400}d ago"
        else -> {
            val days = (diffSec / 86_400).toInt()
            "${days}d ago"
        }
    }
}
