package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class MetadataResponse(
    val id: String,
    @SerialName("device_id") val deviceId: String,
    @SerialName("captured_at") val capturedAt: String,
    val interpretation: String,
    val category: String,
    @SerialName("app_name") val appName: String,
    @SerialName("created_at") val createdAt: String
)

@Serializable
data class MetadataListResponse(
    val metadata: List<MetadataResponse>
)

@Serializable
data class ReconsolidateRequest(
    @SerialName("include_metadata_ids") val includeMetadataIds: List<String>
)

@Serializable
data class PreviewResponse(
    val narrative: String
)
