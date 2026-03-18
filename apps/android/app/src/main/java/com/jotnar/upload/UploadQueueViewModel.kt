package com.jotnar.upload

import android.util.Log
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch
import javax.inject.Inject

data class UploadQueueUiState(
    val items: List<PendingScreenshot> = emptyList(),
    val expandedItemId: String? = null,
    val isUploading: Boolean = false,
    val error: String? = null
)

@HiltViewModel
class UploadQueueViewModel @Inject constructor(
    private val uploadRepository: UploadRepository,
    private val uploadScheduler: UploadScheduler
) : ViewModel() {

    private val _expandedId = MutableStateFlow<String?>(null)
    private val _isUploading = MutableStateFlow(false)
    private val _error = MutableStateFlow<String?>(null)

    val uiState: StateFlow<UploadQueueUiState> = combine(
        uploadRepository.getAllQueued(),
        _expandedId,
        _isUploading,
        _error
    ) { items, expandedId, isUploading, error ->
        UploadQueueUiState(
            items = items,
            expandedItemId = expandedId,
            isUploading = isUploading,
            error = error
        )
    }.stateIn(
        scope = viewModelScope,
        started = SharingStarted.WhileSubscribed(5000),
        initialValue = UploadQueueUiState()
    )

    fun toggleExpand(id: String) {
        _expandedId.update { if (it == id) null else id }
    }

    fun removeItem(id: String) {
        viewModelScope.launch {
            uploadRepository.removeFromQueue(id)
        }
    }

    fun uploadNow() {
        viewModelScope.launch {
            _isUploading.value = true
            _error.value = null
            try {
                while (true) {
                    val result = uploadRepository.uploadBatch()
                    Log.d("UploadQueue", "Batch result: succeeded=${result.succeeded}, failed=${result.failed}, remaining=${result.remainingInQueue}")
                    if (result.remainingInQueue == 0) break
                    if (result.succeeded == 0 && result.failed > 0) {
                        _error.value = "Upload failed — check server connection"
                        break
                    }
                }
            } catch (e: Exception) {
                Log.e("UploadQueue", "Upload error", e)
                _error.value = e.message ?: "Upload failed"
            } finally {
                _isUploading.value = false
            }
        }
    }

    fun clearError() {
        _error.value = null
    }
}
