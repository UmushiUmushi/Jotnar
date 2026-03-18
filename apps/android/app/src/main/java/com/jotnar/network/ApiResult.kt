package com.jotnar.network

import kotlinx.serialization.json.Json
import com.jotnar.network.models.ErrorResponse
import retrofit2.Response

sealed class ApiResult<out T> {
    data class Success<T>(val data: T) : ApiResult<T>()
    data class Error(val code: Int, val message: String) : ApiResult<Nothing>()
    data class NetworkError(val exception: Throwable) : ApiResult<Nothing>()
}

suspend fun <T> safeApiCall(json: Json, call: suspend () -> Response<T>): ApiResult<T> {
    return try {
        val response = call()
        if (response.isSuccessful) {
            val body = response.body()
            if (body != null) {
                ApiResult.Success(body)
            } else {
                ApiResult.Error(response.code(), "Empty response body")
            }
        } else {
            val errorBody = response.errorBody()?.string()
            val message = if (errorBody != null) {
                try {
                    json.decodeFromString<ErrorResponse>(errorBody).error
                } catch (_: Exception) {
                    errorBody
                }
            } else {
                "Unknown error"
            }
            ApiResult.Error(response.code(), message)
        }
    } catch (e: Exception) {
        ApiResult.NetworkError(e)
    }
}
