package com.jotnar.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.jotnar.settings.DevicePreferences
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject

@AndroidEntryPoint
class BootReceiver : BroadcastReceiver() {

    @Inject
    lateinit var devicePreferences: DevicePreferences

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        // Only restart if capture was running before reboot
        // Note: MediaProjection cannot be restarted without user interaction,
        // so we just schedule uploads for any queued screenshots and
        // the user will need to manually restart capture from the app.
        if (devicePreferences.captureWasRunning) {
            // The user needs to re-grant MediaProjection permission,
            // so we can't auto-start capture. We'll just ensure uploads continue.
            val uploadIntent = Intent(context, Class.forName("com.jotnar.upload.UploadScheduler"))
            // WorkManager handles this automatically on boot if there are pending work requests
        }
    }
}
