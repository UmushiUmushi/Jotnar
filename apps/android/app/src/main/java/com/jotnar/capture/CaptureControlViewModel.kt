package com.jotnar.capture

import android.content.Context
import android.content.Intent
import android.provider.Settings
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch
import javax.inject.Inject

data class CaptureControlUiState(
    val captureState: CaptureState = CaptureState.Idle,
    val isServiceEnabled: Boolean = false,
    val queueSize: Int = 0,
    val captureIntervalSec: Int = 10,
    val wifiOnly: Boolean = true
)

@HiltViewModel
class CaptureControlViewModel @Inject constructor(
    private val devicePreferences: DevicePreferences,
    private val uploadRepository: UploadRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(
        CaptureControlUiState(
            captureIntervalSec = devicePreferences.captureIntervalSec,
            wifiOnly = devicePreferences.wifiOnlyUpload
        )
    )
    val uiState: StateFlow<CaptureControlUiState> = _uiState.asStateFlow()

    init {
        viewModelScope.launch {
            JotnarAccessibilityService.captureState.collect { state ->
                _uiState.update { it.copy(captureState = state) }
            }
        }

        viewModelScope.launch {
            JotnarAccessibilityService.isServiceEnabled.collect { enabled ->
                _uiState.update { it.copy(isServiceEnabled = enabled) }
            }
        }

        viewModelScope.launch {
            uploadRepository.queueSize().collect { size ->
                _uiState.update { it.copy(queueSize = size) }
            }
        }
    }

    fun openAccessibilitySettings(context: Context) {
        val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK
        }
        context.startActivity(intent)
    }

    fun startCapture() {
        JotnarAccessibilityService.instance?.startCaptureLoop()
    }

    fun pauseCapture() {
        JotnarAccessibilityService.instance?.pauseCaptureLoop()
    }

    fun stopCapture() {
        JotnarAccessibilityService.instance?.stopCaptureLoop()
    }
}
