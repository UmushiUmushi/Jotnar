package com.jotnar.journal

import com.jotnar.network.ApiResult
import com.jotnar.network.JotnarApi
import com.jotnar.network.models.*
import com.jotnar.network.safeApiCall
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MetadataRepository @Inject constructor(
    private val api: JotnarApi,
    private val json: Json
) {
    suspend fun getMetadata(entryId: String): ApiResult<MetadataListResponse> {
        return safeApiCall(json) { api.getEntryMetadata(entryId) }
    }

    suspend fun previewReconsolidation(
        entryId: String,
        includeIds: List<String>
    ): ApiResult<PreviewResponse> {
        return safeApiCall(json) {
            api.previewReconsolidation(entryId, ReconsolidateRequest(includeIds))
        }
    }

    suspend fun commitReconsolidation(
        entryId: String,
        includeIds: List<String>
    ): ApiResult<JournalEntryResponse> {
        return safeApiCall(json) {
            api.commitReconsolidation(entryId, ReconsolidateRequest(includeIds))
        }
    }
}
