package com.jotnar

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.appcompat.app.AppCompatActivity
import androidx.compose.material3.Surface
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.navigation.compose.rememberNavController
import com.jotnar.auth.TokenStore
import com.jotnar.navigation.JotnarNavGraph
import com.jotnar.settings.DevicePreferences
import com.jotnar.ui.theme.JotnarTheme
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject

@AndroidEntryPoint
class MainActivity : AppCompatActivity() {

    @Inject
    lateinit var tokenStore: TokenStore

    @Inject
    lateinit var devicePreferences: DevicePreferences

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            val themeMode by devicePreferences.themeModeFlow.collectAsState()

            JotnarTheme(themeMode = themeMode) {
                Surface {
                    val navController = rememberNavController()
                    JotnarNavGraph(
                        navController = navController,
                        isAuthenticated = tokenStore.isAuthenticated
                    )
                }
            }
        }
    }
}
