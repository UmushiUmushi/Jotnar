package com.jotnar.di

import android.content.Context
import com.jotnar.upload.UploadDao
import com.jotnar.upload.UploadDatabase
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideUploadDatabase(@ApplicationContext context: Context): UploadDatabase {
        return UploadDatabase.create(context)
    }

    @Provides
    fun provideUploadDao(database: UploadDatabase): UploadDao {
        return database.uploadDao()
    }
}
