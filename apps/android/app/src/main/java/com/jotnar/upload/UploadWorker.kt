package com.jotnar.upload

import android.content.Context
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject

@HiltWorker
class UploadWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted params: WorkerParameters,
    private val uploadRepository: UploadRepository
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        var totalSucceeded = 0
        var totalFailed = 0

        // Drain queue in batches until empty or a full batch fails
        while (true) {
            val result = try {
                uploadRepository.uploadBatch()
            } catch (_: Exception) {
                return if (totalSucceeded > 0) Result.success() else Result.retry()
            }

            totalSucceeded += result.succeeded
            totalFailed += result.failed

            // Stop if queue is empty
            if (result.remainingInQueue == 0) break

            // Stop if a full batch failed (likely a persistent issue)
            if (result.succeeded == 0 && result.failed > 0) break
        }

        return if (totalFailed > 0 && totalSucceeded == 0) {
            Result.retry()
        } else {
            Result.success()
        }
    }
}
