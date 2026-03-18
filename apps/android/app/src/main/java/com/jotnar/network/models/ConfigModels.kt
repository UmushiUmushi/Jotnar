package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class ConfigResponse(
    @SerialName("consolidation_window_min") val consolidationWindowMin: Int,
    @SerialName("interpretation_detail") val interpretationDetail: String,
    @SerialName("journal_tone") val journalTone: String,
    @SerialName("metadata_retention_days") val metadataRetentionDays: Int? = null,
    @SerialName("timezone") val timezone: String = "UTC"
)

@Serializable
data class UpdateConfigRequest(
    @SerialName("consolidation_window_min") val consolidationWindowMin: Int? = null,
    @SerialName("interpretation_detail") val interpretationDetail: String? = null,
    @SerialName("journal_tone") val journalTone: String? = null,
    @SerialName("metadata_retention_days") val metadataRetentionDays: Int? = null,
    @SerialName("timezone") val timezone: String? = null
)
