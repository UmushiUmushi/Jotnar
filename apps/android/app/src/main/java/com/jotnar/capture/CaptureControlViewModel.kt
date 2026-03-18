package com.jotnar.capture

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.media.projection.MediaProjectionManager
import androidx.activity.result.ActivityResultLauncher
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.service.CaptureService
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch
import javax.inject.Inject

data class CaptureControlUiState(
    val captureState: CaptureState = CaptureState.Idle,
    val queueSize: Int = 0,
    val captureIntervalSec: Int = 10,
    val wifiOnly: Boolean = true
)

@HiltViewModel
class CaptureControlViewModel @Inject constructor(
    private val captureScheduler: CaptureScheduler,
    private val screenCaptureManager: ScreenCaptureManager,
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
        // Observe capture state
        viewModelScope.launch {
            captureScheduler.captureState.collect { state ->
                _uiState.update { it.copy(captureState = state) }
            }
        }

        // Observe queue size
        viewModelScope.launch {
            uploadRepository.queueSize().collect { size ->
                _uiState.update { it.copy(queueSize = size) }
            }
        }
    }

    fun requestMediaProjection(activity: Activity, launcher: ActivityResultLauncher<Intent>) {
        val projectionManager = activity.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        launcher.launch(projectionManager.createScreenCaptureIntent())
    }

    fun startCapture(context: Context, resultCode: Int, data: Intent) {
        val serviceIntent = CaptureService.createIntent(context, resultCode, data)
        context.startForegroundService(serviceIntent)
    }

    fun stopCapture() {
        captureScheduler.stop()
        screenCaptureManager.stop()
    }
}
