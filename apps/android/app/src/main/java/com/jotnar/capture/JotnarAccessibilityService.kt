package com.jotnar.capture

import android.accessibilityservice.AccessibilityService
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.graphics.Bitmap
import android.os.BatteryManager
import android.os.PowerManager
import android.view.Display
import android.view.accessibility.AccessibilityEvent
import androidx.core.content.ContextCompat
import com.jotnar.service.CaptureNotificationManager
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadRepository
import com.jotnar.upload.UploadScheduler
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.ByteArrayOutputStream
import java.time.Instant
import javax.inject.Inject
import kotlin.coroutines.resume

@AndroidEntryPoint
class JotnarAccessibilityService : AccessibilityService() {

    @Inject lateinit var appBlocklist: AppBlocklist
    @Inject lateinit var devicePreferences: DevicePreferences
    @Inject lateinit var uploadRepository: UploadRepository
    @Inject lateinit var uploadScheduler: UploadScheduler
    @Inject lateinit var captureNotificationManager: CaptureNotificationManager

    private val serviceScope = CoroutineScope(Dispatchers.Default + SupervisorJob())
    private var captureJob: Job? = null

    companion object {
        var instance: JotnarAccessibilityService? = null
            private set

        private val _captureState = MutableStateFlow(CaptureState.Idle)
        val captureState: StateFlow<CaptureState> = _captureState.asStateFlow()

        private val _isServiceEnabled = MutableStateFlow(false)
        val isServiceEnabled: StateFlow<Boolean> = _isServiceEnabled.asStateFlow()

        private val _currentForegroundApp = MutableStateFlow<String?>(null)
        val currentForegroundApp: StateFlow<String?> = _currentForegroundApp.asStateFlow()
    }

    private var foregroundAppObserverJob: Job? = null

    private val actionReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                CaptureNotificationManager.ACTION_STOP -> stopCaptureLoop()
                CaptureNotificationManager.ACTION_PAUSE -> pauseCaptureLoop()
                CaptureNotificationManager.ACTION_RESUME -> startCaptureLoop()
                CaptureNotificationManager.ACTION_REPOST -> {
                    // Notification was swiped — repost it if capture is active
                    val state = _captureState.value
                    if (state != CaptureState.Idle && state != CaptureState.Stopped) {
                        captureNotificationManager.updateNotification(state)
                    }
                }
            }
        }
    }

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        _isServiceEnabled.value = true

        val filter = IntentFilter().apply {
            addAction(CaptureNotificationManager.ACTION_STOP)
            addAction(CaptureNotificationManager.ACTION_PAUSE)
            addAction(CaptureNotificationManager.ACTION_RESUME)
            addAction(CaptureNotificationManager.ACTION_REPOST)
        }
        ContextCompat.registerReceiver(this, actionReceiver, filter, ContextCompat.RECEIVER_NOT_EXPORTED)

        // Watch foreground app changes to update notification immediately
        foregroundAppObserverJob = serviceScope.launch {
            _currentForegroundApp.collect { foregroundApp ->
                val currentState = _captureState.value
                // Only react if we're in an active capture state (not stopped/idle/manually paused)
                if (currentState != CaptureState.Capturing && currentState != CaptureState.PausedBlockedApp) return@collect

                val isBlocked = foregroundApp != null && appBlocklist.isBlocked(foregroundApp)
                val newState = if (isBlocked) CaptureState.PausedBlockedApp else CaptureState.Capturing
                if (currentState != newState) {
                    _captureState.value = newState
                    updateNotification(newState)
                }
            }
        }

        // Auto-resume capture after reboot or service restart
        if (devicePreferences.captureWasRunning) {
            startCaptureLoop()
        }
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event?.eventType == AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) {
            _currentForegroundApp.value = event.packageName?.toString()
        }
    }

    override fun onInterrupt() {
        // Required override
    }

    override fun onDestroy() {
        foregroundAppObserverJob?.cancel()
        stopCaptureLoop()
        try {
            unregisterReceiver(actionReceiver)
        } catch (_: IllegalArgumentException) {
            // Receiver not registered
        }
        instance = null
        _isServiceEnabled.value = false
        _captureState.value = CaptureState.Idle
        serviceScope.cancel()
        super.onDestroy()
    }

    fun startCaptureLoop() {
        if (captureJob?.isActive == true) return

        devicePreferences.captureWasRunning = true
        uploadScheduler.schedulePeriodicUpload()

        captureJob = serviceScope.launch {
            _captureState.value = CaptureState.Capturing
            updateNotification(CaptureState.Capturing)

            while (isActive) {
                val intervalMs = devicePreferences.captureIntervalSec * 1000L

                val newState = checkPauseConditions()
                if (newState != CaptureState.Capturing) {
                    if (_captureState.value != newState) {
                        _captureState.value = newState
                        updateNotification(newState)
                    }
                    delay(intervalMs)
                    continue
                }

                if (_captureState.value != CaptureState.Capturing) {
                    _captureState.value = CaptureState.Capturing
                    updateNotification(CaptureState.Capturing)
                }

                try {
                    val bitmap = captureScreenshot()
                    if (bitmap != null) {
                        val scaled = scaleBitmap(bitmap, 0.5f)
                        bitmap.recycle()
                        val jpeg = compressToJpeg(scaled)
                        scaled.recycle()
                        uploadRepository.enqueue(jpeg, Instant.now())

                        val queueSize = uploadRepository.queueSizeSync()
                        if (queueSize >= devicePreferences.uploadBatchSize) {
                            uploadScheduler.scheduleImmediateUpload()
                        }
                        updateNotification(CaptureState.Capturing)
                    }
                } catch (_: Exception) {
                    // Silently continue on capture errors
                }

                delay(intervalMs)
            }
        }
    }

    fun pauseCaptureLoop() {
        captureJob?.cancel()
        captureJob = null
        _captureState.value = CaptureState.PausedManual
        captureNotificationManager.updateNotification(CaptureState.PausedManual)
    }

    fun stopCaptureLoop() {
        captureJob?.cancel()
        captureJob = null
        _captureState.value = CaptureState.Stopped
        devicePreferences.captureWasRunning = false
        captureNotificationManager.cancelNotification()
    }

    private suspend fun captureScreenshot(): Bitmap? = suspendCancellableCoroutine { cont ->
        takeScreenshot(
            Display.DEFAULT_DISPLAY,
            mainExecutor,
            object : TakeScreenshotCallback {
                override fun onSuccess(screenshot: ScreenshotResult) {
                    val hardwareBitmap = Bitmap.wrapHardwareBuffer(
                        screenshot.hardwareBuffer,
                        screenshot.colorSpace
                    )
                    screenshot.hardwareBuffer.close()
                    val softwareBitmap = hardwareBitmap?.copy(Bitmap.Config.ARGB_8888, false)
                    hardwareBitmap?.recycle()
                    cont.resume(softwareBitmap)
                }

                override fun onFailure(errorCode: Int) {
                    cont.resume(null)
                }
            }
        )
    }

    private fun scaleBitmap(bitmap: Bitmap, scale: Float): Bitmap {
        val newWidth = (bitmap.width * scale).toInt()
        val newHeight = (bitmap.height * scale).toInt()
        return Bitmap.createScaledBitmap(bitmap, newWidth, newHeight, true)
    }

    private fun compressToJpeg(bitmap: Bitmap): ByteArray {
        val stream = ByteArrayOutputStream()
        bitmap.compress(Bitmap.CompressFormat.JPEG, 80, stream)
        return stream.toByteArray()
    }

    private fun checkPauseConditions(): CaptureState {
        if (devicePreferences.pauseOnBatterySaver) {
            val powerManager = getSystemService(Context.POWER_SERVICE) as? PowerManager
            if (powerManager?.isPowerSaveMode == true) {
                return CaptureState.PausedBatterySaver
            }
        }

        if (devicePreferences.pauseOnLowBattery) {
            val batteryManager = getSystemService(Context.BATTERY_SERVICE) as? BatteryManager
            val level = batteryManager?.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY) ?: 100
            if (level < devicePreferences.lowBatteryThreshold) {
                return CaptureState.PausedLowBattery
            }
        }

        val foregroundApp = _currentForegroundApp.value
        if (foregroundApp != null && appBlocklist.isBlocked(foregroundApp)) {
            return CaptureState.PausedBlockedApp
        }

        return CaptureState.Capturing
    }

    private suspend fun updateNotification(state: CaptureState) {
        val queueSize = try {
            uploadRepository.queueSizeSync()
        } catch (_: Exception) {
            0
        }
        captureNotificationManager.updateNotification(state, queueSize)
    }
}
