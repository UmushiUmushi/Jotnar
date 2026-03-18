package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class CaptureResponse(
    val id: String,
    val interpretation: String,
    val category: String,
    @SerialName("app_name") val appName: String
)

@Serializable
data class BatchCaptureResult(
    val index: Int,
    val id: String? = null,
    val interpretation: String? = null,
    val category: String? = null,
    @SerialName("app_name") val appName: String? = null,
    val error: String? = null
)

@Serializable
data class BatchCaptureResponse(
    val results: List<BatchCaptureResult>,
    val succeeded: Int,
    val failed: Int
)
