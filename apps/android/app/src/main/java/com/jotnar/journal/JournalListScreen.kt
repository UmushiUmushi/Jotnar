package com.jotnar.journal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.expandVertically
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.jotnar.network.models.JournalEntryResponse
import com.jotnar.ui.components.ConfirmationDialog
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun JournalListScreen(
    viewModel: JournalListViewModel = hiltViewModel(),
    onEditEntry: (String) -> Unit,
    onNavigateToSettings: () -> Unit,
    onNavigateToUploadQueue: () -> Unit,
    onNavigateToCaptureControl: () -> Unit
) {
    val state by viewModel.uiState.collectAsState()
    val listState = rememberLazyListState()

    // Load more when near bottom
    val shouldLoadMore by remember {
        derivedStateOf {
            val lastVisibleItem = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0
            lastVisibleItem >= state.entries.size - 5 && state.hasMore && !state.isLoadingMore
        }
    }

    LaunchedEffect(shouldLoadMore) {
        if (shouldLoadMore) viewModel.loadMore()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    if (state.isBatchSelectMode) {
                        Text("${state.selectedEntryIds.size} selected")
                    } else {
                        Text("Jotnar")
                    }
                },
                navigationIcon = {
                    if (state.isBatchSelectMode) {
                        IconButton(onClick = { viewModel.exitBatchSelectMode() }) {
                            Icon(Icons.Default.Close, contentDescription = "Cancel selection")
                        }
                    }
                },
                actions = {
                    if (!state.isBatchSelectMode) {
                        IconButton(onClick = onNavigateToCaptureControl) {
                            Icon(Icons.Default.CameraAlt, contentDescription = "Capture")
                        }
                        IconButton(onClick = onNavigateToUploadQueue) {
                            Icon(Icons.Default.CloudUpload, contentDescription = "Upload queue")
                        }
                        IconButton(onClick = onNavigateToSettings) {
                            Icon(Icons.Default.Settings, contentDescription = "Settings")
                        }
                    }
                }
            )
        },
        floatingActionButton = {
            if (state.isBatchSelectMode && state.selectedEntryIds.isNotEmpty()) {
                ExtendedFloatingActionButton(
                    onClick = { viewModel.batchDelete() },
                    containerColor = MaterialTheme.colorScheme.error,
                    contentColor = MaterialTheme.colorScheme.onError
                ) {
                    if (state.isBatchDeleting) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(20.dp),
                            strokeWidth = 2.dp,
                            color = MaterialTheme.colorScheme.onError
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("Deleting ${state.batchDeleteProgress}/${state.batchDeleteTotal}")
                    } else {
                        Icon(Icons.Default.Delete, contentDescription = null)
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("Delete ${state.selectedEntryIds.size}")
                    }
                }
            }
        }
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.isRefreshing,
            onRefresh = { viewModel.refresh() },
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            if (state.isLoading && state.entries.isEmpty()) {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
            } else if (state.entries.isEmpty() && !state.isLoading) {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(
                            Icons.Default.MenuBook,
                            contentDescription = null,
                            modifier = Modifier.size(64.dp),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        Text(
                            "No journal entries yet",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            "Start capturing to generate entries",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                        )
                    }
                }
            } else {
                val groupedEntries = remember(state.entries, state.zoneId) {
                    state.entries.groupBy { entry ->
                        try {
                            OffsetDateTime.parse(entry.timeStart)
                                .atZoneSameInstant(state.zoneId)
                                .toLocalDate()
                        } catch (_: Exception) {
                            LocalDate.now()
                        }
                    }.toSortedMap(compareByDescending { it })
                }

                LazyColumn(
                    state = listState,
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(0.dp)
                ) {
                    groupedEntries.forEach { (date, entries) ->
                        item(key = "header-$date") {
                            DateHeader(date, state.zoneId)
                        }

                        items(entries, key = { it.id }) { entry ->
                            JournalEntryCard(
                                entry = entry,
                                zoneId = state.zoneId,
                                isExpanded = state.expandedEntryId == entry.id,
                                isBatchSelectMode = state.isBatchSelectMode,
                                isSelected = entry.id in state.selectedEntryIds,
                                onClick = {
                                    if (state.isBatchSelectMode) {
                                        viewModel.toggleEntrySelection(entry.id)
                                    } else {
                                        viewModel.toggleExpandEntry(entry.id)
                                    }
                                },
                                onLongClick = {
                                    if (!state.isBatchSelectMode) {
                                        viewModel.enterBatchSelectMode(entry.id)
                                    }
                                },
                                onEdit = { onEditEntry(entry.id) },
                                onDelete = { viewModel.requestDeleteEntry(entry) }
                            )
                        }
                    }

                    if (state.isLoadingMore) {
                        item {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(16.dp),
                                contentAlignment = Alignment.Center
                            ) {
                                CircularProgressIndicator(modifier = Modifier.size(24.dp))
                            }
                        }
                    }
                }
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

    // Delete confirmation dialog
    if (state.showDeleteConfirmation && state.deleteTargetEntry != null) {
        ConfirmationDialog(
            entryText = state.deleteTargetEntry!!.narrative,
            title = "Delete this entry?",
            description = "This will also delete associated metadata. To keep metadata and regenerate the entry, use the edit page instead.",
            confirmLabel = "Delete",
            onConfirm = { viewModel.confirmDeleteEntry() },
            onDismiss = { viewModel.dismissDeleteConfirmation() },
            isDestructive = true
        )
    }
}

@Composable
private fun DateHeader(date: LocalDate, zoneId: ZoneId) {
    val today = LocalDate.now(zoneId)
    val yesterday = today.minusDays(1)

    val label = when (date) {
        today -> "Today"
        yesterday -> "Yesterday"
        else -> {
            val month = date.month.getDisplayName(JavaTextStyle.FULL, Locale.getDefault())
            "$month ${date.dayOfMonth}"
        }
    }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 16.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        HorizontalDivider(
            modifier = Modifier.weight(1f),
            color = MaterialTheme.colorScheme.outlineVariant
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 16.dp)
        )
        HorizontalDivider(
            modifier = Modifier.weight(1f),
            color = MaterialTheme.colorScheme.outlineVariant
        )
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun JournalEntryCard(
    entry: JournalEntryResponse,
    zoneId: ZoneId,
    isExpanded: Boolean,
    isBatchSelectMode: Boolean,
    isSelected: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
    onEdit: () -> Unit,
    onDelete: () -> Unit
) {
    val containerColor = when {
        isSelected -> MaterialTheme.colorScheme.primaryContainer
        isExpanded -> MaterialTheme.colorScheme.surfaceContainerLow
        else -> Color.Transparent
    }

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .animateContentSize()
            .combinedClickable(
                onClick = onClick,
                onLongClick = onLongClick
            ),
        colors = CardDefaults.cardColors(containerColor = containerColor),
        elevation = CardDefaults.cardElevation(
            defaultElevation = if (isSelected) 2.dp else 0.dp
        )
    ) {
        Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            // Time inline with narrative, text wraps naturally
            Text(
                text = buildAnnotatedString {
                    withStyle(SpanStyle(fontWeight = FontWeight.ExtraBold, fontSize = MaterialTheme.typography.titleSmall.fontSize)) {
                        append(formatStartTime(entry.timeStart, zoneId))
                    }
                    append("  ")
                    append(entry.narrative)
                },
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )

            // Expanded actions (only in non-batch mode)
            AnimatedVisibility(
                visible = isExpanded && !isBatchSelectMode,
                enter = expandVertically(),
                exit = shrinkVertically()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    // Left: edited badge
                    if (entry.edited) {
                        AssistChip(
                            onClick = { },
                            label = { Text("Edited", style = MaterialTheme.typography.labelSmall) },
                            leadingIcon = {
                                Icon(
                                    Icons.Default.Edit,
                                    contentDescription = null,
                                    modifier = Modifier.size(14.dp)
                                )
                            },
                            modifier = Modifier.height(28.dp)
                        )
                    } else {
                        Spacer(modifier = Modifier.width(1.dp))
                    }

                    // Right: edit + delete buttons
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        IconButton(onClick = onEdit) {
                            Icon(
                                Icons.Default.EditNote,
                                contentDescription = "Edit entry",
                                tint = MaterialTheme.colorScheme.primary
                            )
                        }
                        IconButton(onClick = onDelete) {
                            Icon(
                                Icons.Default.Delete,
                                contentDescription = "Delete entry",
                                tint = MaterialTheme.colorScheme.error
                            )
                        }
                    }
                }
            }

            // Batch select: show checkbox
            if (isBatchSelectMode) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 8.dp),
                    horizontalArrangement = Arrangement.End
                ) {
                    Checkbox(
                        checked = isSelected,
                        onCheckedChange = { onClick() }
                    )
                }
            }
        }
    }
}

private fun formatStartTime(startStr: String, zoneId: ZoneId): String {
    return try {
        val start = OffsetDateTime.parse(startStr).atZoneSameInstant(zoneId)
        val timeFormatter = DateTimeFormatter.ofPattern("h:mm a")
        start.format(timeFormatter)
    } catch (_: Exception) {
        startStr
    }
}
