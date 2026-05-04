package com.weiqi.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Extension
import androidx.compose.material.icons.outlined.GridOn
import androidx.compose.material.icons.outlined.Menu
import androidx.compose.material.icons.outlined.RateReview
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.weiqi.ui.screens.GameScreen
import com.weiqi.ui.screens.GameViewModel
import com.weiqi.ui.screens.HomeScreen
import com.weiqi.ui.screens.PuzzlesScreen
import com.weiqi.ui.screens.ReviewScreen
import com.weiqi.ui.screens.SettingsScreen
import com.weiqi.ui.screens.SetupScreen
import com.weiqi.ui.theme.WeiqiTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { WeiqiTheme { AppNav() } }
    }
}

private data class TopTab(val route: String, val label: String, val icon: @Composable () -> Unit)

private val topTabs = listOf(
    TopTab("play", "Play") { Icon(Icons.Outlined.GridOn, contentDescription = "Play") },
    TopTab("puzzles", "Puzzles") { Icon(Icons.Outlined.Extension, contentDescription = "Puzzles") },
    TopTab("review", "Review") { Icon(Icons.Outlined.RateReview, contentDescription = "Review") },
    TopTab("settings", "Settings") { Icon(Icons.Outlined.Settings, contentDescription = "Settings") }
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AppNav() {
    val nav = rememberNavController()
    val gameVm: GameViewModel = viewModel()
    val backStack by nav.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination?.route

    val showChrome = currentRoute in topTabs.map { it.route } || currentRoute == null

    Scaffold(
        topBar = {
            if (showChrome) {
                CenterAlignedTopAppBar(
                    title = {
                        Text(
                            "Weiqi",
                            style = MaterialTheme.typography.headlineSmall,
                            fontWeight = FontWeight.SemiBold
                        )
                    },
                    navigationIcon = {
                        IconButton(onClick = {}) { Icon(Icons.Outlined.Menu, contentDescription = "Menu") }
                    },
                    colors = TopAppBarDefaults.centerAlignedTopAppBarColors(
                        containerColor = MaterialTheme.colorScheme.background
                    )
                )
            }
        },
        bottomBar = {
            if (showChrome) {
                NavigationBar(containerColor = MaterialTheme.colorScheme.surfaceContainer) {
                    topTabs.forEach { tab ->
                        val selected = backStack?.destination?.hierarchy?.any { it.route == tab.route } == true
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
    ) { padding ->
        NavHost(
            navController = nav,
            startDestination = "play",
            modifier = Modifier.padding(padding)
        ) {
            composable("play") {
                HomeScreen(
                    onPlayLocal = { nav.navigate("setup/local") },
                    onPlayAi = { nav.navigate("setup/ai") }
                )
            }
            composable("puzzles") { PuzzlesScreen() }
            composable("review") { ReviewScreen() }
            composable("settings") { SettingsScreen() }
            composable("setup/local") {
                SetupScreen(isAi = false) { cfg, opp, aiColor ->
                    gameVm.startGame(cfg, opp, aiColor)
                    nav.navigate("game")
                }
            }
            composable("setup/ai") {
                SetupScreen(isAi = true) { cfg, opp, aiColor ->
                    gameVm.startGame(cfg, opp, aiColor)
                    nav.navigate("game")
                }
            }
            composable("game") {
                GameScreen(vm = gameVm, onExit = { nav.popBackStack("play", inclusive = false) })
            }
        }
    }
}

