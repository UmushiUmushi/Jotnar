package com.jotnar.capture

import android.Manifest
import android.os.Build
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

    // Request POST_NOTIFICATIONS permission on Android 13+
    val notificationPermissionLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.RequestPermission()
    ) { /* granted or not, proceed — notification is best-effort */ }

    LaunchedEffect(Unit) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
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
            // Accessibility service setup card (shown when service is not enabled)
            if (!state.isServiceEnabled) {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.errorContainer
                    )
                ) {
                    Column(
                        modifier = Modifier.padding(24.dp),
                        horizontalAlignment = Alignment.CenterHorizontally
                    ) {
                        Icon(
                            imageVector = Icons.Default.AccessibilityNew,
                            contentDescription = null,
                            modifier = Modifier.size(48.dp)
                        )

                        Spacer(modifier = Modifier.height(12.dp))

                        Text(
                            text = "Accessibility Service Required",
                            style = MaterialTheme.typography.titleLarge
                        )

                        Spacer(modifier = Modifier.height(8.dp))

                        Text(
                            text = "Jotnar needs the accessibility service enabled to capture screenshots. " +
                                "This is a one-time setup. Open Settings, find Jotnar, and toggle it on.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onErrorContainer
                        )
                    }
                }

                Button(
                    onClick = { viewModel.openAccessibilitySettings(context) },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Icon(Icons.Default.Settings, contentDescription = null)
                    Spacer(modifier = Modifier.width(8.dp))
                    Text("Open Accessibility Settings")
                }
            }

            // Status card (shown when service is enabled)
            if (state.isServiceEnabled) {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(
                        containerColor = when (state.captureState) {
                            CaptureState.Capturing -> MaterialTheme.colorScheme.primaryContainer
                            CaptureState.PausedBlockedApp,
                            CaptureState.PausedBatterySaver,
                            CaptureState.PausedLowBattery,
                            CaptureState.PausedManual -> MaterialTheme.colorScheme.tertiaryContainer
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
                                CaptureState.PausedManual -> Icons.Default.Pause
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
                                CaptureState.PausedManual -> "Paused"
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

                // Control buttons
                when (state.captureState) {
                    CaptureState.Idle, CaptureState.Stopped -> {
                        Button(
                            onClick = { viewModel.startCapture() },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Icon(Icons.Default.PlayArrow, contentDescription = null)
                            Spacer(modifier = Modifier.width(8.dp))
                            Text("Start Capture")
                        }
                    }
                    CaptureState.PausedManual -> {
                        Button(
                            onClick = { viewModel.startCapture() },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Icon(Icons.Default.PlayArrow, contentDescription = null)
                            Spacer(modifier = Modifier.width(8.dp))
                            Text("Resume Capture")
                        }
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
                    else -> {
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
