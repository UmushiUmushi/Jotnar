package com.jotnar.upload

import android.content.Context
import androidx.work.*
import com.jotnar.settings.DevicePreferences
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class UploadScheduler @Inject constructor(
    @ApplicationContext private val context: Context,
    private val devicePreferences: DevicePreferences
) {
    companion object {
        private const val PERIODIC_WORK_NAME = "jotnar_periodic_upload"
        private const val IMMEDIATE_WORK_TAG = "jotnar_immediate_upload"
    }

    fun schedulePeriodicUpload() {
        val constraints = buildConstraints()

        val request = PeriodicWorkRequestBuilder<UploadWorker>(
            15, TimeUnit.MINUTES
        )
            .setConstraints(constraints)
            .setBackoffCriteria(
                BackoffPolicy.EXPONENTIAL,
                WorkRequest.MIN_BACKOFF_MILLIS,
                TimeUnit.MILLISECONDS
            )
            .build()

        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            PERIODIC_WORK_NAME,
            ExistingPeriodicWorkPolicy.KEEP,
            request
        )
    }

    fun scheduleImmediateUpload() {
        val constraints = buildConstraints()

        val request = OneTimeWorkRequestBuilder<UploadWorker>()
            .setConstraints(constraints)
            .setBackoffCriteria(
                BackoffPolicy.EXPONENTIAL,
                WorkRequest.MIN_BACKOFF_MILLIS,
                TimeUnit.MILLISECONDS
            )
            .addTag(IMMEDIATE_WORK_TAG)
            .build()

        // KEEP: if an immediate upload is already running/queued, don't pile up another
        WorkManager.getInstance(context).enqueueUniqueWork(
            IMMEDIATE_WORK_TAG,
            ExistingWorkPolicy.KEEP,
            request
        )
    }

    fun cancelAll() {
        WorkManager.getInstance(context).cancelUniqueWork(PERIODIC_WORK_NAME)
        WorkManager.getInstance(context).cancelUniqueWork(IMMEDIATE_WORK_TAG)
    }

    private fun buildConstraints(): Constraints {
        return Constraints.Builder()
            .setRequiredNetworkType(
                if (devicePreferences.wifiOnlyUpload) {
                    NetworkType.UNMETERED
                } else {
                    NetworkType.CONNECTED
                }
            )
            .build()
    }
}
