package com.jotnar.upload

import android.content.Context
import androidx.room.*

@Entity(tableName = "upload_queue")
data class PendingScreenshot(
    @PrimaryKey val id: String,
    @ColumnInfo(typeAffinity = ColumnInfo.BLOB) val imageData: ByteArray,
    val capturedAt: Long, // epoch millis
    val createdAt: Long,
    val fileSize: Int,
    val retryCount: Int = 0
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is PendingScreenshot) return false
        return id == other.id
    }

    override fun hashCode(): Int = id.hashCode()
}

@Database(entities = [PendingScreenshot::class], version = 1, exportSchema = false)
abstract class UploadDatabase : RoomDatabase() {
    abstract fun uploadDao(): UploadDao

    companion object {
        fun create(context: Context): UploadDatabase {
            return Room.databaseBuilder(
                context,
                UploadDatabase::class.java,
                "jotnar_upload_queue"
            ).build()
        }
    }
}
