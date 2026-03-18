package com.jotnar.settings

import com.jotnar.network.ApiResult
import com.jotnar.network.JotnarApi
import com.jotnar.network.models.ConfigResponse
import com.jotnar.network.models.StatusResponse
import com.jotnar.network.models.UpdateConfigRequest
import com.jotnar.network.safeApiCall
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class SettingsRepository @Inject constructor(
    private val api: JotnarApi,
    private val json: Json
) {
    suspend fun getServerConfig(): ApiResult<ConfigResponse> {
        return safeApiCall(json) { api.getConfig() }
    }

    suspend fun updateServerConfig(request: UpdateConfigRequest): ApiResult<ConfigResponse> {
        return safeApiCall(json) { api.updateConfig(request) }
    }

    suspend fun getServerStatus(): ApiResult<StatusResponse> {
        return safeApiCall(json) { api.getStatus() }
    }
}
