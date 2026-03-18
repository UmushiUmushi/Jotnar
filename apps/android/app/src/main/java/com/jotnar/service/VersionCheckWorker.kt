package com.jotnar.service

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import androidx.core.app.NotificationCompat
import androidx.hilt.work.HiltWorker
import androidx.work.*
import com.jotnar.network.ApiResult
import com.jotnar.settings.SettingsRepository
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import java.util.concurrent.TimeUnit

@HiltWorker
class VersionCheckWorker @AssistedInject constructor(
    @Assisted private val context: Context,
    @Assisted params: WorkerParameters,
    private val settingsRepository: SettingsRepository
) : CoroutineWorker(context, params) {

    companion object {
        private const val CHANNEL_ID = "jotnar_updates"
        private const val NOTIFICATION_ID = 200
        private const val WORK_NAME = "jotnar_version_check"
        private const val APP_VERSION = "0.1.0"

        fun schedule(context: Context) {
            val request = PeriodicWorkRequestBuilder<VersionCheckWorker>(
                1, TimeUnit.DAYS
            ).setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build()
            ).build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.KEEP,
                request
            )
        }
    }

    override suspend fun doWork(): Result {
        when (val result = settingsRepository.getServerStatus()) {
            is ApiResult.Success -> {
                val serverVersion = result.data.version
                if (serverVersion != APP_VERSION) {
                    showUpdateNotification(serverVersion)
                }
            }
            else -> { /* non-critical, skip */ }
        }
        return Result.success()
    }

    private fun showUpdateNotification(serverVersion: String) {
        createChannel()

        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle("Server update available")
            .setContentText("Server is running v$serverVersion, app expects v$APP_VERSION")
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setAutoCancel(true)
            .build()

        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID, notification)
    }

    private fun createChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Update Notifications",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Notifications about server/app version mismatches"
        }
        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.createNotificationChannel(channel)
    }
}
