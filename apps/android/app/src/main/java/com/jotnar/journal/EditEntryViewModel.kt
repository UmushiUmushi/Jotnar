package com.jotnar.journal

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.network.ApiResult
import com.jotnar.network.models.JournalEntryResponse
import com.jotnar.network.models.MetadataResponse
import com.jotnar.settings.TimezoneProvider
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.ZoneId
import javax.inject.Inject

data class EditEntryUiState(
    val entry: JournalEntryResponse? = null,
    val metadata: List<MetadataResponse> = emptyList(),
    val editedNarrative: String = "",
    val toggleStates: Map<String, Boolean> = emptyMap(), // metadata id -> included
    val isRegenerating: Boolean = false,
    val isLoading: Boolean = true,
    val isSaving: Boolean = false,
    val error: String? = null,
    val saved: Boolean = false,
    val showSaveConfirmation: Boolean = false,
    val hasMetadataChanges: Boolean = false,
    val hasNarrativeChanges: Boolean = false,
    val zoneId: ZoneId = ZoneId.of("UTC")
)

@HiltViewModel
class EditEntryViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val journalRepository: JournalRepository,
    private val metadataRepository: MetadataRepository,
    private val timezoneProvider: TimezoneProvider
) : ViewModel() {

    private val entryId: String = savedStateHandle["id"] ?: ""

    private val _uiState = MutableStateFlow(EditEntryUiState(zoneId = timezoneProvider.zoneId))
    val uiState: StateFlow<EditEntryUiState> = _uiState.asStateFlow()

    private var debounceJob: Job? = null
    private var previewJob: Job? = null

    init {
        loadData()
    }

    private fun loadData() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true) }

            // Load entry and metadata in parallel
            val entryResult = journalRepository.getEntry(entryId)
            val metadataResult = metadataRepository.getMetadata(entryId)

            when {
                entryResult is ApiResult.Success && metadataResult is ApiResult.Success -> {
                    val entry = entryResult.data
                    val metadata = metadataResult.data.metadata
                    val toggles = metadata.associate { it.id to true }
                    _uiState.update {
                        it.copy(
                            entry = entry,
                            metadata = metadata,
                            editedNarrative = entry.narrative,
                            toggleStates = toggles,
                            isLoading = false
                        )
                    }
                }
                entryResult is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, error = (entryResult as ApiResult.Error).message)
                    }
                }
                metadataResult is ApiResult.Error -> {
                    // Still show the entry even if metadata fails
                    if (entryResult is ApiResult.Success) {
                        _uiState.update {
                            it.copy(
                                entry = entryResult.data,
                                editedNarrative = entryResult.data.narrative,
                                isLoading = false,
                                error = "Could not load metadata"
                            )
                        }
                    }
                }
                else -> {
                    _uiState.update { it.copy(isLoading = false, error = "Network error") }
                }
            }
        }
    }

    fun updateNarrative(text: String) {
        _uiState.update {
            it.copy(
                editedNarrative = text,
                hasNarrativeChanges = text != it.entry?.narrative
            )
        }
    }

    fun toggleMetadata(id: String) {
        _uiState.update { state ->
            val newToggles = state.toggleStates.toMutableMap().apply {
                this[id] = !(this[id] ?: true)
            }
            val hasChanges = newToggles.any { (_, included) -> !included }
            state.copy(
                toggleStates = newToggles,
                hasMetadataChanges = hasChanges
            )
        }

        // Cancel pending preview and start new debounce
        debounceJob?.cancel()
        previewJob?.cancel()

        debounceJob = viewModelScope.launch {
            delay(1500)
            requestPreview()
        }
    }

    private fun requestPreview() {
        val state = _uiState.value
        val includedIds = state.toggleStates.filter { it.value }.keys.toList()

        if (includedIds.size == state.metadata.size) {
            // All included — revert to original narrative
            _uiState.update {
                it.copy(
                    editedNarrative = it.entry?.narrative ?: "",
                    hasNarrativeChanges = false,
                    isRegenerating = false
                )
            }
            return
        }

        if (includedIds.isEmpty()) {
            _uiState.update {
                it.copy(editedNarrative = "", hasNarrativeChanges = true, isRegenerating = false)
            }
            return
        }

        previewJob = viewModelScope.launch {
            _uiState.update { it.copy(isRegenerating = true) }
            when (val result = metadataRepository.previewReconsolidation(entryId, includedIds)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            editedNarrative = result.data.narrative,
                            hasNarrativeChanges = result.data.narrative != it.entry?.narrative,
                            isRegenerating = false
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(isRegenerating = false, error = "Preview failed: ${result.message}")
                    }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update {
                        it.copy(isRegenerating = false, error = "Preview failed: network error")
                    }
                }
            }
        }
    }

    fun resetToggles() {
        debounceJob?.cancel()
        previewJob?.cancel()
        _uiState.update { state ->
            state.copy(
                toggleStates = state.metadata.associate { it.id to true },
                editedNarrative = state.entry?.narrative ?: "",
                isRegenerating = false,
                hasMetadataChanges = false,
                hasNarrativeChanges = false
            )
        }
    }

    fun requestSave() {
        _uiState.update { it.copy(showSaveConfirmation = true) }
    }

    fun dismissSaveConfirmation() {
        _uiState.update { it.copy(showSaveConfirmation = false) }
    }

    fun confirmSave() {
        val state = _uiState.value
        _uiState.update { it.copy(showSaveConfirmation = false, isSaving = true) }

        viewModelScope.launch {
            val result = if (state.hasMetadataChanges) {
                // Metadata was excluded — reconsolidate to delete excluded rows and rewrite
                val includedIds = state.toggleStates.filter { it.value }.keys.toList()
                metadataRepository.commitReconsolidation(entryId, includedIds)
            } else if (state.hasNarrativeChanges) {
                // Narrative edited (manually or via regenerate) — simple update
                journalRepository.updateNarrative(entryId, state.editedNarrative)
            } else {
                _uiState.update { it.copy(isSaving = false) }
                return@launch
            }

            when (result) {
                is ApiResult.Success -> {
                    _uiState.update { it.copy(isSaving = false, saved = true) }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(isSaving = false, error = "Save failed: ${result.message}") }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(isSaving = false, error = "Save failed: network error") }
                }
            }
        }
    }

    fun regenerate() {
        debounceJob?.cancel()
        previewJob?.cancel()

        val state = _uiState.value
        val includedIds = state.toggleStates.filter { it.value }.keys.toList()

        if (includedIds.isEmpty()) {
            _uiState.update {
                it.copy(editedNarrative = "", hasNarrativeChanges = true, isRegenerating = false)
            }
            return
        }

        previewJob = viewModelScope.launch {
            _uiState.update { it.copy(isRegenerating = true) }
            when (val result = metadataRepository.previewReconsolidation(entryId, includedIds)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            editedNarrative = result.data.narrative,
                            hasNarrativeChanges = result.data.narrative != it.entry?.narrative,
                            isRegenerating = false
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(isRegenerating = false, error = "Regenerate failed: ${result.message}")
                    }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update {
                        it.copy(isRegenerating = false, error = "Regenerate failed: network error")
                    }
                }
            }
        }
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }
}
