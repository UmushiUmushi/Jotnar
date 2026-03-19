package com.jotnar.upload

import android.content.Context
import androidx.room.*
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase

@Entity(tableName = "upload_queue")
data class PendingScreenshot(
    @PrimaryKey val id: String,
    @ColumnInfo(typeAffinity = ColumnInfo.BLOB) val imageData: ByteArray,
    val capturedAt: Long, // epoch millis
    val createdAt: Long,
    val fileSize: Int,
    val appName: String = "",
    val retryCount: Int = 0
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is PendingScreenshot) return false
        return id == other.id
    }

    override fun hashCode(): Int = id.hashCode()
}

@Database(entities = [PendingScreenshot::class], version = 2, exportSchema = false)
abstract class UploadDatabase : RoomDatabase() {
    abstract fun uploadDao(): UploadDao

    companion object {
        private val MIGRATION_1_2 = object : Migration(1, 2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                db.execSQL("ALTER TABLE upload_queue ADD COLUMN appName TEXT NOT NULL DEFAULT ''")
            }
        }

        fun create(context: Context): UploadDatabase {
            return Room.databaseBuilder(
                context,
                UploadDatabase::class.java,
                "jotnar_upload_queue"
            ).addMigrations(MIGRATION_1_2).build()
        }
    }
}
