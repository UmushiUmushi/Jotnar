package com.jotnar.auth

import com.jotnar.network.ApiResult
import com.jotnar.network.JotnarApi
import com.jotnar.network.models.*
import com.jotnar.network.safeApiCall
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthRepository @Inject constructor(
    private val api: JotnarApi,
    private val tokenStore: TokenStore,
    private val serverPreferences: ServerPreferences,
    private val json: Json
) {
    suspend fun checkServerStatus(): ApiResult<StatusResponse> {
        return safeApiCall(json) { api.getStatus() }
    }

    fun saveServerAddress(address: String) {
        serverPreferences.serverAddress = address
    }

    suspend fun pair(serverAddress: String, code: String, deviceName: String): ApiResult<PairResponse> {
        serverPreferences.serverAddress = serverAddress
        val result = safeApiCall(json) {
            api.pair(PairRequest(code = code, deviceName = deviceName))
        }
        if (result is ApiResult.Success) {
            tokenStore.save(result.data.token, result.data.deviceId)
        }
        return result
    }

    suspend fun recover(serverAddress: String, recoveryKey: String): ApiResult<RecoverResponse> {
        serverPreferences.serverAddress = serverAddress
        return safeApiCall(json) {
            api.recover(RecoverRequest(recoveryKey = recoveryKey))
        }
    }

    suspend fun generatePairingCode(): ApiResult<PairingCodeResponse> {
        return safeApiCall(json) { api.generatePairingCode() }
    }

    fun logout() {
        tokenStore.clear()
        serverPreferences.serverAddress = null
    }

    val isAuthenticated: Boolean
        get() = tokenStore.isAuthenticated
}
