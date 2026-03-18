package com.jotnar.capture

import android.content.Context
import android.graphics.Bitmap
import android.os.BatteryManager
import android.os.PowerManager
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.ByteArrayOutputStream
import java.time.Instant
import javax.inject.Inject
import javax.inject.Singleton

enum class CaptureState {
    Idle,
    Capturing,
    PausedBlockedApp,
    PausedBatterySaver,
    PausedLowBattery,
    Stopped
}

@Singleton
class CaptureScheduler @Inject constructor(
    @ApplicationContext private val context: Context,
    private val screenCaptureManager: ScreenCaptureManager,
    private val foregroundAppDetector: ForegroundAppDetector,
    private val appBlocklist: AppBlocklist,
    private val devicePreferences: DevicePreferences,
    private val uploadRepository: UploadRepository,
    private val uploadScheduler: com.jotnar.upload.UploadScheduler
) {
    private var captureJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.Default + SupervisorJob())

    private val _captureState = MutableStateFlow(CaptureState.Idle)
    val captureState: StateFlow<CaptureState> = _captureState.asStateFlow()

    var onStateChanged: ((CaptureState) -> Unit)? = null

    fun start() {
        if (captureJob?.isActive == true) return

        captureJob = scope.launch {
            _captureState.value = CaptureState.Capturing
            onStateChanged?.invoke(CaptureState.Capturing)

            while (isActive) {
                val intervalMs = devicePreferences.captureIntervalSec * 1000L

                // Check pause conditions
                val newState = checkPauseConditions()
                if (newState != CaptureState.Capturing) {
                    if (_captureState.value != newState) {
                        _captureState.value = newState
                        onStateChanged?.invoke(newState)
                    }
                    delay(intervalMs)
                    continue
                }

                if (_captureState.value != CaptureState.Capturing) {
                    _captureState.value = CaptureState.Capturing
                    onStateChanged?.invoke(CaptureState.Capturing)
                }

                // Capture
                try {
                    val bitmap = screenCaptureManager.captureScreenshot()
                    if (bitmap != null) {
                        val jpeg = compressToJpeg(bitmap)
                        bitmap.recycle()
                        uploadRepository.enqueue(jpeg, Instant.now())

                        // Trigger upload when a batch is ready
                        val queueSize = uploadRepository.queueSizeSync()
                        if (queueSize >= devicePreferences.uploadBatchSize) {
                            uploadScheduler.scheduleImmediateUpload()
                        }
                    }
                } catch (_: Exception) {
                    // Silently continue on capture errors
                }

                delay(intervalMs)
            }
        }
    }

    fun stop() {
        captureJob?.cancel()
        captureJob = null
        _captureState.value = CaptureState.Stopped
        onStateChanged?.invoke(CaptureState.Stopped)
    }

    private fun checkPauseConditions(): CaptureState {
        // Check battery saver
        if (devicePreferences.pauseOnBatterySaver) {
            val powerManager = context.getSystemService(Context.POWER_SERVICE) as? PowerManager
            if (powerManager?.isPowerSaveMode == true) {
                return CaptureState.PausedBatterySaver
            }
        }

        // Check low battery
        if (devicePreferences.pauseOnLowBattery) {
            val batteryManager = context.getSystemService(Context.BATTERY_SERVICE) as? BatteryManager
            val level = batteryManager?.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY) ?: 100
            if (level < devicePreferences.lowBatteryThreshold) {
                return CaptureState.PausedLowBattery
            }
        }

        // Check blocked app
        val foregroundApp = foregroundAppDetector.getCurrentForegroundApp()
        if (foregroundApp != null && appBlocklist.isBlocked(foregroundApp)) {
            return CaptureState.PausedBlockedApp
        }

        return CaptureState.Capturing
    }

    private fun compressToJpeg(bitmap: Bitmap): ByteArray {
        val stream = ByteArrayOutputStream()
        bitmap.compress(Bitmap.CompressFormat.JPEG, 80, stream)
        return stream.toByteArray()
    }
}
