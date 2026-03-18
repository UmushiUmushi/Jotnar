package com.jotnar.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.jotnar.ui.theme.ThemeMode
import java.util.TimeZone

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    viewModel: SettingsViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit,
    onNavigateToBlocklist: () -> Unit,
    onLogout: () -> Unit
) {
    val state by viewModel.uiState.collectAsState()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            // Capture settings
            item { SectionHeader("Capture") }

            item {
                NumericFieldSetting(
                    label = "Capture interval",
                    value = state.captureIntervalSec,
                    suffix = "s",
                    onValueChange = { viewModel.setCaptureInterval(it) },
                    valueRange = 1..300
                )
            }

            item {
                SwitchSetting(
                    label = "Pause on battery saver",
                    checked = state.pauseOnBatterySaver,
                    onCheckedChange = { viewModel.setPauseOnBatterySaver(it) }
                )
            }

            item {
                SwitchSetting(
                    label = "Pause on low battery",
                    checked = state.pauseOnLowBattery,
                    onCheckedChange = { viewModel.setPauseOnLowBattery(it) }
                )
            }

            if (state.pauseOnLowBattery) {
                item {
                    SliderSetting(
                        label = "Low battery threshold",
                        value = state.lowBatteryThreshold.toFloat(),
                        valueRange = 5f..50f,
                        steps = 8,
                        valueLabel = "${state.lowBatteryThreshold}%",
                        onValueChange = { viewModel.setLowBatteryThreshold(it.toInt()) }
                    )
                }
            }

            item {
                SwitchSetting(
                    label = "Wi-Fi only upload",
                    checked = state.wifiOnlyUpload,
                    onCheckedChange = { viewModel.setWifiOnlyUpload(it) }
                )
            }

            item {
                SwitchSetting(
                    label = "Weekly capture reminder",
                    checked = state.captureReminderEnabled,
                    onCheckedChange = { viewModel.setCaptureReminderEnabled(it) }
                )
            }

            // Privacy
            item { SectionHeader("Privacy") }

            item {
                ListItem(
                    headlineContent = { Text("App blocklist") },
                    supportingContent = { Text("${viewModel.uiState.value.run { "" }}Manage blocked apps") },
                    trailingContent = {
                        Icon(Icons.Default.ChevronRight, contentDescription = null)
                    },
                    modifier = Modifier.clickable { onNavigateToBlocklist() }
                )
            }

            // Theme
            item { SectionHeader("Theme") }

            item { ThemeModeSelector(state.themeMode, viewModel::setThemeMode) }

            // Server config
            item { SectionHeader("Server") }

            if (state.isLoadingServerConfig) {
                item {
                    Box(modifier = Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(modifier = Modifier.size(24.dp))
                    }
                }
            }

            state.serverConfig?.let { config ->
                item {
                    DropdownSetting(
                        label = "Interpretation detail",
                        value = config.interpretationDetail,
                        options = listOf("minimal", "standard", "detailed"),
                        onValueChange = { viewModel.updateServerConfig(interpretationDetail = it) }
                    )
                }

                item {
                    DropdownSetting(
                        label = "Journal tone",
                        value = config.journalTone,
                        options = listOf("casual", "concise", "narrative"),
                        onValueChange = { viewModel.updateServerConfig(journalTone = it) }
                    )
                }

                item {
                    TimezoneDropdownSetting(
                        value = config.timezone,
                        onValueChange = { viewModel.updateServerConfig(timezone = it) }
                    )
                }

                item {
                    NumericFieldSetting(
                        label = "Consolidation window",
                        value = config.consolidationWindowMin,
                        suffix = "min",
                        onValueChange = {
                            viewModel.updateServerConfig(consolidationWindowMin = it)
                        },
                        valueRange = 1..1440
                    )
                }
            }

            // Device management
            item { SectionHeader("Device") }

            item {
                ListItem(
                    headlineContent = { Text("Server address") },
                    supportingContent = { Text(state.serverAddress) }
                )
            }

            state.serverVersion?.let { version ->
                item {
                    ListItem(
                        headlineContent = { Text("Server version") },
                        supportingContent = { Text(version) },
                        trailingContent = {
                            if (state.modelAvailable) {
                                Icon(
                                    Icons.Default.CheckCircle,
                                    contentDescription = "Model ready",
                                    tint = MaterialTheme.colorScheme.primary
                                )
                            }
                        }
                    )
                }
            }

            item {
                ListItem(
                    headlineContent = { Text("Pair new device") },
                    leadingContent = {
                        Icon(Icons.Default.DevicesOther, contentDescription = null)
                    },
                    modifier = Modifier.clickable { viewModel.generatePairingCode() }
                )
            }

            item {
                ListItem(
                    headlineContent = {
                        Text("Unpair this device", color = MaterialTheme.colorScheme.error)
                    },
                    leadingContent = {
                        Icon(
                            Icons.AutoMirrored.Filled.Logout,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.error
                        )
                    },
                    modifier = Modifier.clickable {
                        viewModel.logout()
                        onLogout()
                    }
                )
            }

            // About
            item { SectionHeader("About") }

            item {
                ListItem(
                    headlineContent = { Text("Jotnar") },
                    supportingContent = { Text("Version 0.1.0") }
                )
            }
        }
    }

    // Pairing code dialog
    state.generatedPairingCode?.let { code ->
        AlertDialog(
            onDismissRequest = { viewModel.dismissPairingCode() },
            title = { Text("Pairing code") },
            text = {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        "Share this code with the new device:",
                        style = MaterialTheme.typography.bodyMedium
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(
                        text = code,
                        style = MaterialTheme.typography.displaySmall,
                        fontWeight = FontWeight.Bold,
                        fontFamily = FontFamily.Monospace,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "Expires in 10 minutes",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = { viewModel.dismissPairingCode() }) {
                    Text("Done")
                }
            }
        )
    }

    // Error snackbar
    if (state.error != null) {
        Snackbar(
            modifier = Modifier.padding(16.dp),
            action = {
                TextButton(onClick = { viewModel.clearError() }) {
                    Text("Dismiss")
                }
            }
        ) {
            Text(state.error!!)
        }
    }
}

@Composable
private fun SectionHeader(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleSmall,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(top = 16.dp, bottom = 4.dp)
    )
}

@Composable
private fun SwitchSetting(
    label: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
) {
    ListItem(
        headlineContent = { Text(label) },
        trailingContent = {
            Switch(checked = checked, onCheckedChange = onCheckedChange)
        }
    )
}

@Composable
private fun SliderSetting(
    label: String,
    value: Float,
    valueRange: ClosedFloatingPointRange<Float>,
    steps: Int,
    valueLabel: String,
    onValueChange: (Float) -> Unit
) {
    Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(label, style = MaterialTheme.typography.bodyLarge)
            Text(
                valueLabel,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.primary
            )
        }
        Slider(
            value = value,
            onValueChange = onValueChange,
            valueRange = valueRange,
            steps = steps
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DropdownSetting(
    label: String,
    value: String,
    options: List<String>,
    onValueChange: (String) -> Unit
) {
    var expanded by remember { mutableStateOf(false) }

    ExposedDropdownMenuBox(
        expanded = expanded,
        onExpandedChange = { expanded = it },
        modifier = Modifier.padding(horizontal = 16.dp)
    ) {
        OutlinedTextField(
            value = value.replaceFirstChar { it.uppercase() },
            onValueChange = { },
            readOnly = true,
            label = { Text(label) },
            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded) },
            modifier = Modifier
                .fillMaxWidth()
                .menuAnchor()
        )
        ExposedDropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            options.forEach { option ->
                DropdownMenuItem(
                    text = { Text(option.replaceFirstChar { it.uppercase() }) },
                    onClick = {
                        expanded = false
                        if (option != value) onValueChange(option)
                    },
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TimezoneDropdownSetting(
    value: String,
    onValueChange: (String) -> Unit
) {
    var expanded by remember { mutableStateOf(false) }
    var searchQuery by remember { mutableStateOf("") }
    val allTimezones = remember { TimeZone.getAvailableIDs().sorted() }
    val filteredTimezones = remember(searchQuery) {
        if (searchQuery.isBlank()) allTimezones
        else allTimezones.filter { it.contains(searchQuery, ignoreCase = true) }
    }

    ExposedDropdownMenuBox(
        expanded = expanded,
        onExpandedChange = { expanded = it },
        modifier = Modifier.padding(horizontal = 16.dp)
    ) {
        OutlinedTextField(
            value = value,
            onValueChange = { },
            readOnly = true,
            label = { Text("Timezone") },
            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded) },
            modifier = Modifier
                .fillMaxWidth()
                .menuAnchor()
        )
        ExposedDropdownMenu(
            expanded = expanded,
            onDismissRequest = {
                expanded = false
                searchQuery = ""
            }
        ) {
            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                placeholder = { Text("Search...") },
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 8.dp, vertical = 4.dp)
            )
            filteredTimezones.take(50).forEach { tz ->
                DropdownMenuItem(
                    text = { Text(tz) },
                    onClick = {
                        onValueChange(tz)
                        expanded = false
                        searchQuery = ""
                    },
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)
                )
            }
        }
    }
}

@Composable
private fun ThemeModeSelector(current: ThemeMode, onSelect: (ThemeMode) -> Unit) {
    Column(modifier = Modifier.padding(horizontal = 16.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            ThemeMode.entries.forEach { mode ->
                FilterChip(
                    selected = current == mode,
                    onClick = { onSelect(mode) },
                    label = {
                        Text(
                            when (mode) {
                                ThemeMode.Light -> "Light"
                                ThemeMode.Dark -> "Dark"
                                ThemeMode.MaterialYou -> "Dynamic"
                            },
                            style = MaterialTheme.typography.labelLarge
                        )
                    }
                )
            }
        }
    }
}

@Composable
private fun NumericFieldSetting(
    label: String,
    value: Int,
    suffix: String,
    onValueChange: (Int) -> Unit,
    valueRange: IntRange
) {
    var textValue by remember(value) { mutableStateOf(value.toString()) }
    val focusManager = LocalFocusManager.current

    fun commitValue() {
        val parsed = textValue.toIntOrNull()
        if (parsed != null && parsed in valueRange) {
            onValueChange(parsed)
        } else {
            textValue = value.toString()
        }
    }

    ListItem(
        headlineContent = { Text(label) },
        trailingContent = {
            OutlinedTextField(
                value = textValue,
                onValueChange = { newText ->
                    if (newText.all { it.isDigit() }) {
                        textValue = newText
                    }
                },
                suffix = { Text(suffix, style = MaterialTheme.typography.bodyLarge) },
                textStyle = MaterialTheme.typography.bodyLarge,
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                keyboardActions = KeyboardActions(onDone = {
                    commitValue()
                    focusManager.clearFocus()
                }),
                modifier = Modifier
                    .width(100.dp)
                    .onFocusChanged { if (!it.isFocused) commitValue() }
            )
        }
    )
}

