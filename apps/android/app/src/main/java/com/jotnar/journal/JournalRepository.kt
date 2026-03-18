package com.jotnar.journal

import com.jotnar.network.ApiResult
import com.jotnar.network.JotnarApi
import com.jotnar.network.models.*
import com.jotnar.network.safeApiCall
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class JournalRepository @Inject constructor(
    private val api: JotnarApi,
    private val json: Json
) {
    suspend fun getEntries(limit: Int = 20, offset: Int = 0): ApiResult<JournalListResponse> {
        return safeApiCall(json) { api.listJournal(limit, offset) }
    }

    suspend fun getEntry(id: String): ApiResult<JournalEntryResponse> {
        return safeApiCall(json) { api.getJournalEntry(id) }
    }

    suspend fun updateNarrative(id: String, narrative: String): ApiResult<JournalEntryResponse> {
        return safeApiCall(json) { api.updateJournalEntry(id, UpdateJournalRequest(narrative)) }
    }

    suspend fun deleteEntry(id: String): ApiResult<DeleteResponse> {
        return safeApiCall(json) { api.deleteJournalEntry(id) }
    }
}
