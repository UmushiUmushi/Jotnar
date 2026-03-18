package com.jotnar.network.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class PairRequest(
    val code: String,
    @SerialName("device_name") val deviceName: String
)

@Serializable
data class PairResponse(
    @SerialName("device_id") val deviceId: String,
    val token: String
)

@Serializable
data class RecoverRequest(
    @SerialName("recovery_key") val recoveryKey: String
)

@Serializable
data class RecoverResponse(
    @SerialName("pairing_code") val pairingCode: String
)

@Serializable
data class PairingCodeResponse(
    val code: String
)
