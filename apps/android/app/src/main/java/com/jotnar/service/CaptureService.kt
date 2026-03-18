package com.jotnar.service

import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.IBinder
import androidx.core.content.ContextCompat
import com.jotnar.capture.CaptureScheduler
import com.jotnar.capture.CaptureState
import com.jotnar.capture.ScreenCaptureManager
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadRepository
import com.jotnar.upload.UploadScheduler
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.collectLatest
import javax.inject.Inject

@AndroidEntryPoint
class CaptureService : Service() {

    @Inject lateinit var screenCaptureManager: ScreenCaptureManager
    @Inject lateinit var captureScheduler: CaptureScheduler
    @Inject lateinit var captureNotificationManager: CaptureNotificationManager
    @Inject lateinit var devicePreferences: DevicePreferences
    @Inject lateinit var uploadRepository: UploadRepository
    @Inject lateinit var uploadScheduler: UploadScheduler

    private val serviceScope = CoroutineScope(Dispatchers.Default + SupervisorJob())

    private val actionReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                CaptureNotificationManager.ACTION_STOP -> {
                    stopCapture()
                    stopSelf()
                }
                CaptureNotificationManager.ACTION_PAUSE -> {
                    captureScheduler.stop()
                }
                CaptureNotificationManager.ACTION_RESUME -> {
                    captureScheduler.start()
                }
            }
        }
    }

    companion object {
        const val EXTRA_RESULT_CODE = "result_code"
        const val EXTRA_DATA = "data"

        fun createIntent(context: Context, resultCode: Int, data: Intent): Intent {
            return Intent(context, CaptureService::class.java).apply {
                putExtra(EXTRA_RESULT_CODE, resultCode)
                putExtra(EXTRA_DATA, data)
            }
        }
    }

    override fun onCreate() {
        super.onCreate()
        val filter = IntentFilter().apply {
            addAction(CaptureNotificationManager.ACTION_STOP)
            addAction(CaptureNotificationManager.ACTION_PAUSE)
            addAction(CaptureNotificationManager.ACTION_RESUME)
        }
        ContextCompat.registerReceiver(this, actionReceiver, filter, ContextCompat.RECEIVER_NOT_EXPORTED)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val resultCode = intent?.getIntExtra(EXTRA_RESULT_CODE, 0) ?: 0
        val data = intent?.getParcelableExtra<Intent>(EXTRA_DATA)

        // Start foreground immediately
        val notification = captureNotificationManager.buildNotification(CaptureState.Idle)
        startForeground(CaptureNotificationManager.NOTIFICATION_ID, notification)

        if (data != null && resultCode != 0) {
            // Start MediaProjection
            screenCaptureManager.start(resultCode, data) {
                // On revocation
                stopCapture()
                stopSelf()
            }

            // Start capture loop
            captureScheduler.start()
            devicePreferences.captureWasRunning = true

            // Monitor state changes for notification updates
            captureScheduler.onStateChanged = { state ->
                serviceScope.launch {
                    val queueSize = uploadRepository.queueSizeSync()
                    captureNotificationManager.updateNotification(state, queueSize)
                }
            }

            // Schedule uploads
            uploadScheduler.schedulePeriodicUpload()
        }

        return START_NOT_STICKY
    }

    private fun stopCapture() {
        captureScheduler.stop()
        screenCaptureManager.stop()
        devicePreferences.captureWasRunning = false
    }

    override fun onDestroy() {
        stopCapture()
        unregisterReceiver(actionReceiver)
        serviceScope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}
