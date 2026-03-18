package com.jotnar.service

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import androidx.core.app.NotificationCompat
import androidx.hilt.work.HiltWorker
import androidx.work.*
import com.jotnar.settings.DevicePreferences
import com.jotnar.upload.UploadDao
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import java.util.concurrent.TimeUnit

@HiltWorker
class CaptureReminderWorker @AssistedInject constructor(
    @Assisted private val context: Context,
    @Assisted params: WorkerParameters,
    private val devicePreferences: DevicePreferences,
    private val uploadDao: UploadDao
) : CoroutineWorker(context, params) {

    companion object {
        private const val CHANNEL_ID = "jotnar_reminders"
        private const val NOTIFICATION_ID = 100
        private const val WORK_NAME = "jotnar_capture_reminder"

        fun schedule(context: Context) {
            val request = PeriodicWorkRequestBuilder<CaptureReminderWorker>(
                7, TimeUnit.DAYS
            ).build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.KEEP,
                request
            )
        }

        fun cancel(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork(WORK_NAME)
        }
    }

    override suspend fun doWork(): Result {
        if (!devicePreferences.captureReminderEnabled) return Result.success()

        val queueSize = uploadDao.count()

        createChannel()

        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setContentTitle("Jotnar weekly summary")
            .setContentText("$queueSize screenshots queued for upload")
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setAutoCancel(true)
            .build()

        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID, notification)

        return Result.success()
    }

    private fun createChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Weekly Reminders",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Weekly capture activity reminders"
        }
        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.createNotificationChannel(channel)
    }
}
