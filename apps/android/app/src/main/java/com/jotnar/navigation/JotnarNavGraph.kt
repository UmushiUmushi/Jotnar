package com.jotnar.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.jotnar.auth.AuthScreen
import com.jotnar.capture.CaptureControlScreen
import com.jotnar.journal.EditEntryScreen
import com.jotnar.journal.JournalListScreen
import com.jotnar.settings.AppBlocklistScreen
import com.jotnar.settings.SettingsScreen
import com.jotnar.upload.UploadQueueScreen

object Routes {
    const val AUTH = "auth"
    const val JOURNAL_LIST = "journal_list"
    const val JOURNAL_EDIT = "journal_edit/{id}"
    const val UPLOAD_QUEUE = "upload_queue"
    const val SETTINGS = "settings"
    const val BLOCKLIST = "blocklist"
    const val CAPTURE_CONTROL = "capture_control"

    fun journalEdit(id: String) = "journal_edit/$id"
}

@Composable
fun JotnarNavGraph(
    navController: NavHostController,
    isAuthenticated: Boolean
) {
    NavHost(
        navController = navController,
        startDestination = if (isAuthenticated) Routes.JOURNAL_LIST else Routes.AUTH
    ) {
        composable(Routes.AUTH) {
            AuthScreen(
                onPaired = {
                    navController.navigate(Routes.JOURNAL_LIST) {
                        popUpTo(Routes.AUTH) { inclusive = true }
                    }
                }
            )
        }

        composable(Routes.JOURNAL_LIST) {
            JournalListScreen(
                onEditEntry = { id -> navController.navigate(Routes.journalEdit(id)) },
                onNavigateToSettings = { navController.navigate(Routes.SETTINGS) },
                onNavigateToUploadQueue = { navController.navigate(Routes.UPLOAD_QUEUE) },
                onNavigateToCaptureControl = { navController.navigate(Routes.CAPTURE_CONTROL) }
            )
        }

        composable(
            route = Routes.JOURNAL_EDIT,
            arguments = listOf(navArgument("id") { type = NavType.StringType })
        ) { backStackEntry ->
            val entryId = backStackEntry.arguments?.getString("id") ?: return@composable
            EditEntryScreen(
                entryId = entryId,
                onNavigateBack = { navController.popBackStack() }
            )
        }

        composable(Routes.UPLOAD_QUEUE) {
            UploadQueueScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }

        composable(Routes.SETTINGS) {
            SettingsScreen(
                onNavigateBack = { navController.popBackStack() },
                onNavigateToBlocklist = { navController.navigate(Routes.BLOCKLIST) },
                onLogout = {
                    navController.navigate(Routes.AUTH) {
                        popUpTo(0) { inclusive = true }
                    }
                }
            )
        }

        composable(Routes.BLOCKLIST) {
            AppBlocklistScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }

        composable(Routes.CAPTURE_CONTROL) {
            CaptureControlScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }
    }
}
