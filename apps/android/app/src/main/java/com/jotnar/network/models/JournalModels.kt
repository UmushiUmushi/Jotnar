package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class JournalEntryResponse(
    val id: String,
    val narrative: String,
    @SerialName("time_start") val timeStart: String,
    @SerialName("time_end") val timeEnd: String,
    val edited: Boolean,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String? = null
)

@Serializable
data class JournalListResponse(
    val entries: List<JournalEntryResponse>,
    val total: Int,
    val limit: Int,
    val offset: Int
)

@Serializable
data class UpdateJournalRequest(
    val narrative: String
)
