package com.jotnar.journal

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Save
import androidx.compose.material.icons.filled.Undo
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.jotnar.network.models.MetadataResponse
import com.jotnar.ui.components.ConfirmationDialog
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditEntryScreen(
    entryId: String,
    viewModel: EditEntryViewModel = hiltViewModel(),
    onNavigateBack: (saved: Boolean) -> Unit
) {
    val state by viewModel.uiState.collectAsState()

    LaunchedEffect(state.saved) {
        if (state.saved) onNavigateBack(true)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Edit entry") },
                navigationIcon = {
                    IconButton(onClick = { onNavigateBack(false) }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    if (state.hasMetadataChanges) {
                        IconButton(onClick = { viewModel.resetToggles() }) {
                            Icon(Icons.Default.Undo, contentDescription = "Reset toggles")
                        }
                    }
                    IconButton(
                        onClick = { viewModel.regenerate() },
                        enabled = !state.isRegenerating && !state.isSaving
                    ) {
                        Icon(Icons.Default.Refresh, contentDescription = "Regenerate")
                    }
                    IconButton(
                        onClick = { viewModel.requestSave() },
                        enabled = (state.hasNarrativeChanges || state.hasMetadataChanges) && !state.isSaving
                    ) {
                        Icon(Icons.Default.Save, contentDescription = "Save")
                    }
                }
            )
        }
    ) { padding ->
        if (state.isLoading) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
            return@Scaffold
        }

        if (state.isSaving) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    CircularProgressIndicator()
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("Saving...", style = MaterialTheme.typography.bodyMedium)
                }
            }
            return@Scaffold
        }

        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Narrative editor
            item {
                Text(
                    text = "Narrative",
                    style = MaterialTheme.typography.titleMedium,
                    modifier = Modifier.padding(bottom = 4.dp)
                )
                OutlinedTextField(
                    value = state.editedNarrative,
                    onValueChange = { viewModel.updateNarrative(it) },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !state.isRegenerating,
                    minLines = 3,
                    maxLines = 10
                )
                if (state.isRegenerating) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(top = 8.dp)
                    ) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(16.dp),
                            strokeWidth = 2.dp
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Text(
                            "Regenerating...",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            // Metadata section header
            if (state.metadata.isNotEmpty()) {
                item {
                    HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))
                    Text(
                        text = "Metadata (${state.toggleStates.count { it.value }}/${state.metadata.size} included)",
                        style = MaterialTheme.typography.titleMedium,
                        modifier = Modifier.padding(bottom = 4.dp)
                    )
                }
            }

            // Metadata rows
            items(state.metadata, key = { it.id }) { meta ->
                val isIncluded = state.toggleStates[meta.id] ?: true
                MetadataRow(
                    metadata = meta,
                    zoneId = state.zoneId,
                    isIncluded = isIncluded,
                    onToggle = { viewModel.toggleMetadata(meta.id) }
                )
            }
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

    // Save confirmation dialog
    if (state.showSaveConfirmation) {
        ConfirmationDialog(
            entryText = state.editedNarrative,
            title = "Save changes?",
            confirmLabel = "Save",
            onConfirm = { viewModel.confirmSave() },
            onDismiss = { viewModel.dismissSaveConfirmation() }
        )
    }
}

@Composable
private fun MetadataRow(
    metadata: MetadataResponse,
    zoneId: ZoneId,
    isIncluded: Boolean,
    onToggle: () -> Unit
) {
    val backgroundColor by animateColorAsState(
        targetValue = if (isIncluded) {
            MaterialTheme.colorScheme.surfaceContainerLow
        } else {
            MaterialTheme.colorScheme.surfaceContainerLow.copy(alpha = 0.4f)
        },
        label = "metadata_bg"
    )

    val contentAlpha = if (isIncluded) 1f else 0.4f

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = backgroundColor)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(
                modifier = Modifier.weight(1f)
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Text(
                        text = formatTimestamp(metadata.capturedAt, zoneId),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = contentAlpha)
                    )
                    if (metadata.appName.isNotBlank()) {
                        SuggestionChip(
                            onClick = { },
                            label = {
                                Text(
                                    metadata.appName,
                                    style = MaterialTheme.typography.labelSmall
                                )
                            },
                            modifier = Modifier.height(24.dp)
                        )
                    }
                    SuggestionChip(
                        onClick = { },
                        label = {
                            Text(
                                metadata.category,
                                style = MaterialTheme.typography.labelSmall
                            )
                        },
                        modifier = Modifier.height(24.dp)
                    )
                }
                Spacer(modifier = Modifier.height(4.dp))
                var expanded by remember { mutableStateOf(false) }
                Text(
                    text = metadata.interpretation,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = contentAlpha),
                    maxLines = if (expanded) Int.MAX_VALUE else 2,
                    overflow = if (expanded) TextOverflow.Clip else TextOverflow.Ellipsis,
                    modifier = Modifier
                        .animateContentSize()
                        .clickable { expanded = !expanded }
                )
            }

            Switch(
                checked = isIncluded,
                onCheckedChange = { onToggle() },
                modifier = Modifier.padding(start = 8.dp)
            )
        }
    }
}

private fun formatTimestamp(timestamp: String, zoneId: ZoneId): String {
    return try {
        val dt = OffsetDateTime.parse(timestamp).atZoneSameInstant(zoneId)
        dt.format(DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT))
    } catch (_: Exception) {
        timestamp
    }
}
