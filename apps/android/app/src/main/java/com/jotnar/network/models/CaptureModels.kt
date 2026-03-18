package com.jotnar.network.models

import kotlinx.serialization.Serializable

@Serializable
data class CaptureResponse(
    val accepted: Int
)

@Serializable
data class BatchCaptureResult(
    val index: Int,
    val error: String? = null
)

@Serializable
data class BatchCaptureResponse(
    val accepted: Int,
    val rejected: Int,
    val results: List<BatchCaptureResult>? = null
)
