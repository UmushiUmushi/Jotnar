package com.jotnar.upload

import android.util.Log
import com.jotnar.network.JotnarApi
import com.jotnar.settings.DevicePreferences
import kotlinx.coroutines.flow.Flow
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

data class UploadResult(
    val succeeded: Int,
    val failed: Int,
    val remainingInQueue: Int
)

@Singleton
class UploadRepository @Inject constructor(
    private val uploadDao: UploadDao,
    private val api: JotnarApi,
    private val devicePreferences: DevicePreferences
) {
    suspend fun enqueue(imageData: ByteArray, capturedAt: Instant) {
        val screenshot = PendingScreenshot(
            id = UUID.randomUUID().toString(),
            imageData = imageData,
            capturedAt = capturedAt.toEpochMilli(),
            createdAt = System.currentTimeMillis(),
            fileSize = imageData.size
        )
        uploadDao.insert(screenshot)
    }

    suspend fun uploadBatch(): UploadResult {
        val batchSize = devicePreferences.uploadBatchSize
        val batch = uploadDao.getOldestBatch(batchSize)

        if (batch.isEmpty()) {
            return UploadResult(0, 0, 0)
        }

        return try {
            if (batch.size == 1) {
                uploadSingle(batch[0])
            } else {
                uploadMultiple(batch)
            }
        } catch (e: Exception) {
            Log.e("Upload", "uploadBatch exception: ${e.javaClass.simpleName}: ${e.message}", e)
            for (item in batch) {
                uploadDao.incrementRetry(item.id)
            }
            // Clean up items that have failed too many times
            uploadDao.deleteOldFailures(10)
            val remaining = uploadDao.count()
            UploadResult(0, batch.size, remaining)
        }
    }

    private suspend fun uploadSingle(screenshot: PendingScreenshot): UploadResult {
        val part = MultipartBody.Part.createFormData(
            "screenshot",
            "screenshot.jpg",
            screenshot.imageData.toRequestBody("image/jpeg".toMediaType())
        )
        val timestamp = Instant.ofEpochMilli(screenshot.capturedAt)
            .atOffset(ZoneOffset.UTC)
            .format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)
            .toRequestBody("text/plain".toMediaType())

        val response = api.capture(part, timestamp)
        return if (response.isSuccessful) {
            uploadDao.deleteById(screenshot.id)
            val remaining = uploadDao.count()
            UploadResult(1, 0, remaining)
        } else {
            val errorBody = response.errorBody()?.string()
            Log.e("Upload", "Single upload failed: ${response.code()} — $errorBody")
            uploadDao.incrementRetry(screenshot.id)
            val remaining = uploadDao.count()
            UploadResult(0, 1, remaining)
        }
    }

    private suspend fun uploadMultiple(batch: List<PendingScreenshot>): UploadResult {
        val parts = batch.map { screenshot ->
            MultipartBody.Part.createFormData(
                "screenshots",
                "screenshot_${screenshot.id}.jpg",
                screenshot.imageData.toRequestBody("image/jpeg".toMediaType())
            )
        }

        val timestamps = batch.map { screenshot ->
            Instant.ofEpochMilli(screenshot.capturedAt)
                .atOffset(ZoneOffset.UTC)
                .format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)
                .toRequestBody("text/plain".toMediaType())
        }

        val response = api.batchCapture(parts, timestamps)
        val body = response.body()

        if (!response.isSuccessful) {
            val errorBody = response.errorBody()?.string()
            Log.e("Upload", "Batch upload failed: ${response.code()} — $errorBody")
        }

        return if (response.isSuccessful && body != null) {
            body.results.filter { it.error != null }.forEach { result ->
                Log.e("Upload", "Batch item ${result.index} failed: ${result.error}")
            }

            val succeededIds = body.results
                .filter { it.error == null && it.id != null }
                .mapNotNull { result -> batch.getOrNull(result.index)?.id }

            if (succeededIds.isNotEmpty()) {
                uploadDao.deleteByIds(succeededIds)
            }

            // Increment retry for failed items
            val failedIds = batch.map { it.id }.filter { it !in succeededIds }
            for (id in failedIds) {
                uploadDao.incrementRetry(id)
            }

            val remaining = uploadDao.count()
            UploadResult(body.succeeded, body.failed, remaining)
        } else {
            for (item in batch) {
                uploadDao.incrementRetry(item.id)
            }
            val remaining = uploadDao.count()
            UploadResult(0, batch.size, remaining)
        }
    }

    fun queueSize(): Flow<Int> = uploadDao.countFlow()

    suspend fun queueSizeSync(): Int = uploadDao.count()

    fun getAllQueued(): Flow<List<PendingScreenshot>> = uploadDao.getAll()

    suspend fun removeFromQueue(id: String) {
        uploadDao.deleteById(id)
    }

    suspend fun getScreenshot(id: String): PendingScreenshot? {
        return uploadDao.getById(id)
    }
}
