package com.jotnar.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.graphics.Color
import androidx.core.app.NotificationCompat
import com.jotnar.MainActivity
import com.jotnar.R
import com.jotnar.capture.CaptureState
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CaptureNotificationManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        const val CHANNEL_ID = "jotnar_capture_v4"
        const val NOTIFICATION_ID = 1
        const val ACTION_PAUSE = "com.jotnar.ACTION_PAUSE"
        const val ACTION_RESUME = "com.jotnar.ACTION_RESUME"
        const val ACTION_STOP = "com.jotnar.ACTION_STOP"
        const val ACTION_REPOST = "com.jotnar.ACTION_REPOST"

        private const val COLOR_ACTIVE = 0xFF4CAF50.toInt()  // Green
        private const val COLOR_PAUSED = 0xFFF44336.toInt()  // Red
        private const val COLOR_IDLE = 0xFF9E9E9E.toInt()    // Grey
    }

    private val notificationManager =
        context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

    init {
        createChannel()
    }

    private fun createChannel() {
        // Delete old channels
        notificationManager.deleteNotificationChannel("jotnar_capture")
        notificationManager.deleteNotificationChannel("jotnar_capture_v2")
        notificationManager.deleteNotificationChannel("jotnar_capture_v3")

        val channel = NotificationChannel(
            CHANNEL_ID,
            "Screen Capture",
            NotificationManager.IMPORTANCE_DEFAULT
        ).apply {
            description = "Shows when Jotnar is capturing screenshots"
            setShowBadge(false)
            setSound(null, null)
            enableVibration(false)
        }
        notificationManager.createNotificationChannel(channel)
    }

    fun buildNotification(state: CaptureState, queueSize: Int = 0): Notification {
        val contentIntent = PendingIntent.getActivity(
            context,
            0,
            Intent(context, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )

        val title = when (state) {
            CaptureState.Capturing -> "Capturing"
            CaptureState.PausedBlockedApp -> "Paused \u2014 blocked app"
            CaptureState.PausedBatterySaver -> "Paused \u2014 battery saver"
            CaptureState.PausedLowBattery -> "Paused \u2014 low battery"
            CaptureState.PausedManual -> "Paused"
            CaptureState.Stopped -> "Stopped"
            CaptureState.Idle -> "Ready"
        }

        // Expanded text — only shown when notification is pulled down
        val expandedText = when (state) {
            CaptureState.Capturing -> {
                val queueText = if (queueSize > 0) "$queueSize screenshots queued for upload" else "Jotnar is capturing screenshots"
                queueText
            }
            CaptureState.PausedBlockedApp -> "Capture paused while blocked app is in foreground"
            CaptureState.PausedBatterySaver -> "Capture paused while battery saver is active"
            CaptureState.PausedLowBattery -> "Capture paused due to low battery"
            CaptureState.PausedManual -> "Capture is paused"
            CaptureState.Stopped -> "Screen capture is not active"
            CaptureState.Idle -> "Tap to start capturing"
        }

        val (icon, color) = when (state) {
            CaptureState.Capturing -> R.drawable.ic_capture_active to COLOR_ACTIVE
            CaptureState.PausedBlockedApp,
            CaptureState.PausedBatterySaver,
            CaptureState.PausedLowBattery,
            CaptureState.PausedManual -> R.drawable.ic_capture_paused to COLOR_PAUSED
            CaptureState.Stopped, CaptureState.Idle -> R.drawable.ic_capture_paused to COLOR_IDLE
        }

        val builder = NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(expandedText)
            .setSmallIcon(icon)
            .setColor(color)
            .setContentIntent(contentIntent)
            .setOngoing(true)
            .setSilent(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setDeleteIntent(createActionIntent(ACTION_REPOST))

        // Add action buttons based on state
        when (state) {
            CaptureState.Capturing -> {
                builder.addAction(
                    NotificationCompat.Action.Builder(
                        android.R.drawable.ic_media_pause,
                        "Pause",
                        createActionIntent(ACTION_PAUSE)
                    ).build()
                )
                builder.addAction(
                    NotificationCompat.Action.Builder(
                        android.R.drawable.ic_menu_close_clear_cancel,
                        "Stop",
                        createActionIntent(ACTION_STOP)
                    ).build()
                )
            }
            CaptureState.PausedManual -> {
                builder.addAction(
                    NotificationCompat.Action.Builder(
                        android.R.drawable.ic_media_play,
                        "Resume",
                        createActionIntent(ACTION_RESUME)
                    ).build()
                )
                builder.addAction(
                    NotificationCompat.Action.Builder(
                        android.R.drawable.ic_menu_close_clear_cancel,
                        "Stop",
                        createActionIntent(ACTION_STOP)
                    ).build()
                )
            }
            CaptureState.PausedBlockedApp, CaptureState.PausedBatterySaver, CaptureState.PausedLowBattery -> {
                builder.addAction(
                    NotificationCompat.Action.Builder(
                        android.R.drawable.ic_menu_close_clear_cancel,
                        "Stop",
                        createActionIntent(ACTION_STOP)
                    ).build()
                )
            }
            else -> { }
        }

        return builder.build()
    }

    fun updateNotification(state: CaptureState, queueSize: Int = 0) {
        notificationManager.notify(NOTIFICATION_ID, buildNotification(state, queueSize))
    }

    fun cancelNotification() {
        notificationManager.cancel(NOTIFICATION_ID)
    }

    private fun createActionIntent(action: String): PendingIntent {
        val intent = Intent(action).setPackage(context.packageName)
        return PendingIntent.getBroadcast(
            context,
            action.hashCode(),
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
    }
}
