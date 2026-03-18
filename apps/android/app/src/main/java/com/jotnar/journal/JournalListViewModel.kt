package com.jotnar.journal

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.network.ApiResult
import com.jotnar.network.models.JournalEntryResponse
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class JournalListUiState(
    val entries: List<JournalEntryResponse> = emptyList(),
    val isLoading: Boolean = false,
    val isLoadingMore: Boolean = false,
    val isRefreshing: Boolean = false,
    val hasMore: Boolean = true,
    val error: String? = null,
    val expandedEntryId: String? = null,
    // Batch select
    val isBatchSelectMode: Boolean = false,
    val selectedEntryIds: Set<String> = emptySet(),
    val isBatchDeleting: Boolean = false,
    val batchDeleteProgress: Int = 0,
    val batchDeleteTotal: Int = 0,
    // Confirmation dialog
    val showDeleteConfirmation: Boolean = false,
    val deleteTargetEntry: JournalEntryResponse? = null,
    val total: Int = 0
)

@HiltViewModel
class JournalListViewModel @Inject constructor(
    private val journalRepository: JournalRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(JournalListUiState())
    val uiState: StateFlow<JournalListUiState> = _uiState.asStateFlow()

    private val pageSize = 20

    init {
        loadEntries()
    }

    fun loadEntries() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            when (val result = journalRepository.getEntries(limit = pageSize, offset = 0)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            entries = result.data.entries,
                            total = result.data.total,
                            hasMore = result.data.entries.size < result.data.total,
                            isLoading = false
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(isLoading = false, hasMore = false, error = result.message) }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(isLoading = false, hasMore = false, error = "Network error") }
                }
            }
        }
    }

    fun loadMore() {
        val state = _uiState.value
        if (state.isLoadingMore || !state.hasMore) return

        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingMore = true) }
            val offset = state.entries.size
            when (val result = journalRepository.getEntries(limit = pageSize, offset = offset)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        val existingIds = it.entries.map { e -> e.id }.toSet()
                        val newEntries = result.data.entries.filter { e -> e.id !in existingIds }
                        val allEntries = it.entries + newEntries
                        it.copy(
                            entries = allEntries,
                            total = result.data.total,
                            hasMore = allEntries.size < result.data.total,
                            isLoadingMore = false
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(isLoadingMore = false, hasMore = false, error = result.message) }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(isLoadingMore = false, hasMore = false, error = "Network error") }
                }
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            _uiState.update { it.copy(isRefreshing = true, error = null, hasMore = false) }
            when (val result = journalRepository.getEntries(limit = pageSize, offset = 0)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            entries = result.data.entries,
                            total = result.data.total,
                            hasMore = result.data.entries.size < result.data.total,
                            isRefreshing = false
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(isRefreshing = false, error = result.message) }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(isRefreshing = false, error = "Network error") }
                }
            }
        }
    }

    fun toggleExpandEntry(id: String) {
        if (_uiState.value.isBatchSelectMode) return
        _uiState.update {
            it.copy(expandedEntryId = if (it.expandedEntryId == id) null else id)
        }
    }

    // Batch select

    fun enterBatchSelectMode(entryId: String) {
        _uiState.update {
            it.copy(
                isBatchSelectMode = true,
                selectedEntryIds = setOf(entryId),
                expandedEntryId = null
            )
        }
    }

    fun exitBatchSelectMode() {
        _uiState.update {
            it.copy(isBatchSelectMode = false, selectedEntryIds = emptySet())
        }
    }

    fun toggleEntrySelection(id: String) {
        _uiState.update { state ->
            val newSelection = if (id in state.selectedEntryIds) {
                state.selectedEntryIds - id
            } else {
                state.selectedEntryIds + id
            }
            if (newSelection.isEmpty()) {
                state.copy(isBatchSelectMode = false, selectedEntryIds = emptySet())
            } else {
                state.copy(selectedEntryIds = newSelection)
            }
        }
    }

    // Delete

    fun requestDeleteEntry(entry: JournalEntryResponse) {
        _uiState.update {
            it.copy(showDeleteConfirmation = true, deleteTargetEntry = entry)
        }
    }

    fun dismissDeleteConfirmation() {
        _uiState.update {
            it.copy(showDeleteConfirmation = false, deleteTargetEntry = null)
        }
    }

    fun confirmDeleteEntry() {
        val entry = _uiState.value.deleteTargetEntry ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(showDeleteConfirmation = false, deleteTargetEntry = null) }
            when (journalRepository.deleteEntry(entry.id)) {
                is ApiResult.Success -> {
                    _uiState.update { state ->
                        state.copy(
                            entries = state.entries.filter { it.id != entry.id },
                            total = state.total - 1,
                            expandedEntryId = null
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(error = "Failed to delete entry") }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(error = "Network error") }
                }
            }
        }
    }

    fun batchDelete() {
        val ids = _uiState.value.selectedEntryIds.toList()
        if (ids.isEmpty()) return

        viewModelScope.launch {
            _uiState.update {
                it.copy(
                    isBatchDeleting = true,
                    batchDeleteProgress = 0,
                    batchDeleteTotal = ids.size
                )
            }
            val deletedIds = mutableSetOf<String>()
            for ((index, id) in ids.withIndex()) {
                when (journalRepository.deleteEntry(id)) {
                    is ApiResult.Success -> deletedIds.add(id)
                    else -> { /* continue with remaining */ }
                }
                _uiState.update { it.copy(batchDeleteProgress = index + 1) }
            }
            _uiState.update { state ->
                state.copy(
                    entries = state.entries.filter { it.id !in deletedIds },
                    total = state.total - deletedIds.size,
                    isBatchSelectMode = false,
                    selectedEntryIds = emptySet(),
                    isBatchDeleting = false
                )
            }
        }
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }
}
