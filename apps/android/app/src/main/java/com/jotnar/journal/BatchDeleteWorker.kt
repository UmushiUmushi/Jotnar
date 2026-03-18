package com.jotnar.journal

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import androidx.core.app.NotificationCompat
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import com.jotnar.network.ApiResult
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject

@HiltWorker
class BatchDeleteWorker @AssistedInject constructor(
    @Assisted private val context: Context,
    @Assisted params: WorkerParameters,
    private val journalRepository: JournalRepository
) : CoroutineWorker(context, params) {

    companion object {
        const val KEY_ENTRY_IDS = "entry_ids"
        private const val CHANNEL_ID = "jotnar_batch_ops"
        private const val NOTIFICATION_ID = 300
    }

    override suspend fun doWork(): Result {
        val ids = inputData.getStringArray(KEY_ENTRY_IDS) ?: return Result.failure()

        createChannel()

        var succeeded = 0
        var failed = 0

        for ((index, id) in ids.withIndex()) {
            updateNotification(index + 1, ids.size)

            when (journalRepository.deleteEntry(id)) {
                is ApiResult.Success -> succeeded++
                else -> failed++
            }
        }

        // Final notification
        val finalNotification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_delete)
            .setContentTitle("Batch delete complete")
            .setContentText("Deleted $succeeded entries" + if (failed > 0) ", $failed failed" else "")
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setAutoCancel(true)
            .build()

        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID, finalNotification)

        return Result.success(
            workDataOf("succeeded" to succeeded, "failed" to failed)
        )
    }

    private fun updateNotification(current: Int, total: Int) {
        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_delete)
            .setContentTitle("Deleting entries")
            .setContentText("Deleting $current of $total...")
            .setProgress(total, current, false)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .setSilent(true)
            .build()

        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID, notification)
    }

    private fun createChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Batch Operations",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Progress notifications for batch operations"
        }
        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.createNotificationChannel(channel)
    }
}
