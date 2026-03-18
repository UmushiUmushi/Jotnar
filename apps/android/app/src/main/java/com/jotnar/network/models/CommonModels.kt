package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class StatusResponse(
    val version: String,
    @SerialName("model_available") val modelAvailable: Boolean,
    @SerialName("device_count") val deviceCount: Int
)

@Serializable
data class ErrorResponse(
    val error: String
)

@Serializable
data class DeleteResponse(
    val deleted: Boolean
)
