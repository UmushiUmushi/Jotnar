package com.jotnar.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import com.jotnar.MainActivity
import com.jotnar.capture.CaptureState
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CaptureNotificationManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        const val CHANNEL_ID = "jotnar_capture"
        const val NOTIFICATION_ID = 1
        const val ACTION_PAUSE = "com.jotnar.ACTION_PAUSE"
        const val ACTION_RESUME = "com.jotnar.ACTION_RESUME"
        const val ACTION_STOP = "com.jotnar.ACTION_STOP"
    }

    private val notificationManager =
        context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

    init {
        createChannel()
    }

    private fun createChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Screen Capture",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Shows when Jotnar is capturing screenshots"
            setShowBadge(false)
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

        val (title, text) = when (state) {
            CaptureState.Capturing -> {
                val queueText = if (queueSize > 0) " ($queueSize queued)" else ""
                "Capturing..." to "Jotnar is recording your screen$queueText"
            }
            CaptureState.PausedBlockedApp -> "Paused \u2014 blocked app" to "Capture paused while blocked app is in foreground"
            CaptureState.PausedBatterySaver -> "Paused \u2014 battery saver" to "Capture paused while battery saver is active"
            CaptureState.PausedLowBattery -> "Paused \u2014 low battery" to "Capture paused due to low battery"
            CaptureState.Stopped -> "Stopped" to "Screen capture is not active"
            CaptureState.Idle -> "Ready" to "Tap to start capturing"
        }

        val builder = NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setContentIntent(contentIntent)
            .setOngoing(true)
            .setSilent(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)

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
