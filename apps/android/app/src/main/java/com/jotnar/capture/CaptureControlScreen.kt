package com.jotnar.capture

import android.app.Activity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CaptureControlScreen(
    viewModel: CaptureControlViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit
) {
    val state by viewModel.uiState.collectAsState()
    val context = LocalContext.current
    val activity = context as? Activity

    val projectionLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK && result.data != null) {
            viewModel.startCapture(context, result.resultCode, result.data!!)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Capture") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(24.dp)
        ) {
            // Status card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    containerColor = when (state.captureState) {
                        CaptureState.Capturing -> MaterialTheme.colorScheme.primaryContainer
                        CaptureState.PausedBlockedApp,
                        CaptureState.PausedBatterySaver,
                        CaptureState.PausedLowBattery -> MaterialTheme.colorScheme.tertiaryContainer
                        CaptureState.Stopped, CaptureState.Idle -> MaterialTheme.colorScheme.surfaceVariant
                    }
                )
            ) {
                Column(
                    modifier = Modifier.padding(24.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Icon(
                        imageVector = when (state.captureState) {
                            CaptureState.Capturing -> Icons.Default.CameraAlt
                            CaptureState.PausedBlockedApp -> Icons.Default.Block
                            CaptureState.PausedBatterySaver -> Icons.Default.BatterySaver
                            CaptureState.PausedLowBattery -> Icons.Default.BatteryAlert
                            CaptureState.Stopped, CaptureState.Idle -> Icons.Default.CameraAlt
                        },
                        contentDescription = null,
                        modifier = Modifier.size(48.dp)
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    Text(
                        text = when (state.captureState) {
                            CaptureState.Capturing -> "Capturing"
                            CaptureState.PausedBlockedApp -> "Paused \u2014 Blocked App"
                            CaptureState.PausedBatterySaver -> "Paused \u2014 Battery Saver"
                            CaptureState.PausedLowBattery -> "Paused \u2014 Low Battery"
                            CaptureState.Stopped -> "Stopped"
                            CaptureState.Idle -> "Ready"
                        },
                        style = MaterialTheme.typography.titleLarge
                    )

                    if (state.queueSize > 0) {
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text = "${state.queueSize} screenshots queued",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            // Start/Stop button
            if (state.captureState == CaptureState.Idle || state.captureState == CaptureState.Stopped) {
                Button(
                    onClick = {
                        if (activity != null) {
                            viewModel.requestMediaProjection(activity, projectionLauncher)
                        }
                    },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Icon(Icons.Default.PlayArrow, contentDescription = null)
                    Spacer(modifier = Modifier.width(8.dp))
                    Text("Start Capture")
                }
            } else {
                OutlinedButton(
                    onClick = { viewModel.stopCapture() },
                    modifier = Modifier.fillMaxWidth(),
                    colors = ButtonDefaults.outlinedButtonColors(
                        contentColor = MaterialTheme.colorScheme.error
                    )
                ) {
                    Icon(Icons.Default.Stop, contentDescription = null)
                    Spacer(modifier = Modifier.width(8.dp))
                    Text("Stop Capture")
                }
            }

            // Info card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceContainerLow
                )
            ) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Text(
                        "Capture settings",
                        style = MaterialTheme.typography.titleSmall
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text("Interval", style = MaterialTheme.typography.bodyMedium)
                        Text(
                            "${state.captureIntervalSec}s",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text("Wi-Fi only upload", style = MaterialTheme.typography.bodyMedium)
                        Text(
                            if (state.wifiOnly) "Yes" else "No",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }
            }
        }
    }
}
