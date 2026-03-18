package com.jotnar.upload

import androidx.room.*
import kotlinx.coroutines.flow.Flow

@Dao
interface UploadDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(screenshot: PendingScreenshot)

    @Query("SELECT * FROM upload_queue ORDER BY capturedAt ASC LIMIT :limit")
    suspend fun getOldestBatch(limit: Int): List<PendingScreenshot>

    @Query("DELETE FROM upload_queue WHERE id IN (:ids)")
    suspend fun deleteByIds(ids: List<String>)

    @Query("SELECT COUNT(*) FROM upload_queue")
    suspend fun count(): Int

    @Query("SELECT COUNT(*) FROM upload_queue")
    fun countFlow(): Flow<Int>

    @Query("UPDATE upload_queue SET retryCount = retryCount + 1 WHERE id = :id")
    suspend fun incrementRetry(id: String)

    @Query("DELETE FROM upload_queue WHERE retryCount >= :maxRetries")
    suspend fun deleteOldFailures(maxRetries: Int)

    @Query("SELECT * FROM upload_queue ORDER BY capturedAt DESC")
    fun getAll(): Flow<List<PendingScreenshot>>

    @Query("SELECT * FROM upload_queue WHERE id = :id")
    suspend fun getById(id: String): PendingScreenshot?

    @Query("DELETE FROM upload_queue WHERE id = :id")
    suspend fun deleteById(id: String)
}
