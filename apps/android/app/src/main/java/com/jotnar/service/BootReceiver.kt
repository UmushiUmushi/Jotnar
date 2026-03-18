package com.jotnar.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class BootReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        // The AccessibilityService auto-restarts after reboot if enabled in system settings.
        // It checks devicePreferences.captureWasRunning in onServiceConnected() and resumes
        // the capture loop automatically. WorkManager also persists pending upload work
        // across reboots. This receiver is kept as a safety net but is effectively a no-op.
    }
}
