package com.jotnar.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.auth.AuthRepository
import com.jotnar.auth.ServerPreferences
import com.jotnar.network.ApiResult
import com.jotnar.network.models.ConfigResponse
import com.jotnar.network.models.UpdateConfigRequest
import com.jotnar.ui.theme.ThemeMode
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.TimeZone
import javax.inject.Inject

data class SettingsUiState(
    // Device settings
    val captureIntervalSec: Int = 10,
    val pauseOnBatterySaver: Boolean = true,
    val pauseOnLowBattery: Boolean = false,
    val lowBatteryThreshold: Int = 15,
    val wifiOnlyUpload: Boolean = true,
    val notificationStyle: String = "persistent",
    val captureReminderEnabled: Boolean = true,
    val uploadBatchSize: Int = 10,
    // Theme
    val themeMode: ThemeMode = ThemeMode.MaterialYou,
    // Server config
    val serverConfig: ConfigResponse? = null,
    val isLoadingServerConfig: Boolean = false,
    // Server info
    val serverAddress: String = "",
    val serverVersion: String? = null,
    val modelAvailable: Boolean = false,
    // Pairing
    val generatedPairingCode: String? = null,
    // General
    val error: String? = null,
    val isSavingServerConfig: Boolean = false
)

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val devicePreferences: DevicePreferences,
    private val settingsRepository: SettingsRepository,
    private val authRepository: AuthRepository,
    private val serverPreferences: ServerPreferences,
    private val timezoneProvider: TimezoneProvider
) : ViewModel() {

    private val _uiState = MutableStateFlow(loadDeviceSettings())
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        loadServerConfig()
        loadServerStatus()
    }

    private fun loadDeviceSettings(): SettingsUiState {
        return SettingsUiState(
            captureIntervalSec = devicePreferences.captureIntervalSec,
            pauseOnBatterySaver = devicePreferences.pauseOnBatterySaver,
            pauseOnLowBattery = devicePreferences.pauseOnLowBattery,
            lowBatteryThreshold = devicePreferences.lowBatteryThreshold,
            wifiOnlyUpload = devicePreferences.wifiOnlyUpload,
            notificationStyle = devicePreferences.notificationStyle,
            captureReminderEnabled = devicePreferences.captureReminderEnabled,
            uploadBatchSize = devicePreferences.uploadBatchSize,
            themeMode = devicePreferences.themeMode,
            serverAddress = serverPreferences.serverAddress ?: ""
        )
    }

    private fun loadServerConfig() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingServerConfig = true) }
            when (val result = settingsRepository.getServerConfig()) {
                is ApiResult.Success -> {
                    val config = result.data
                    timezoneProvider.update(config.timezone)
                    _uiState.update {
                        it.copy(serverConfig = config, isLoadingServerConfig = false)
                    }
                    // If timezone is still the server default, push the device timezone
                    if (config.timezone == "UTC") {
                        val deviceTz = TimeZone.getDefault().id
                        if (deviceTz != "UTC") {
                            updateServerConfig(timezone = deviceTz)
                        }
                    }
                }
                else -> {
                    _uiState.update {
                        it.copy(isLoadingServerConfig = false, error = "Failed to load server config")
                    }
                }
            }
        }
    }

    private fun loadServerStatus() {
        viewModelScope.launch {
            when (val result = settingsRepository.getServerStatus()) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            serverVersion = result.data.version,
                            modelAvailable = result.data.modelAvailable
                        )
                    }
                }
                else -> { /* non-critical */ }
            }
        }
    }

    // Device settings updates (apply immediately)

    fun setCaptureInterval(seconds: Int) {
        devicePreferences.captureIntervalSec = seconds
        _uiState.update { it.copy(captureIntervalSec = seconds) }
    }

    fun setPauseOnBatterySaver(enabled: Boolean) {
        devicePreferences.pauseOnBatterySaver = enabled
        _uiState.update { it.copy(pauseOnBatterySaver = enabled) }
    }

    fun setPauseOnLowBattery(enabled: Boolean) {
        devicePreferences.pauseOnLowBattery = enabled
        _uiState.update { it.copy(pauseOnLowBattery = enabled) }
    }

    fun setLowBatteryThreshold(threshold: Int) {
        devicePreferences.lowBatteryThreshold = threshold
        _uiState.update { it.copy(lowBatteryThreshold = threshold) }
    }

    fun setWifiOnlyUpload(enabled: Boolean) {
        devicePreferences.wifiOnlyUpload = enabled
        _uiState.update { it.copy(wifiOnlyUpload = enabled) }
    }

    fun setNotificationStyle(style: String) {
        devicePreferences.notificationStyle = style
        _uiState.update { it.copy(notificationStyle = style) }
    }

    fun setCaptureReminderEnabled(enabled: Boolean) {
        devicePreferences.captureReminderEnabled = enabled
        _uiState.update { it.copy(captureReminderEnabled = enabled) }
    }

    fun setUploadBatchSize(size: Int) {
        devicePreferences.uploadBatchSize = size
        _uiState.update { it.copy(uploadBatchSize = size) }
    }

    // Theme

    fun setThemeMode(mode: ThemeMode) {
        devicePreferences.themeMode = mode
        _uiState.update { it.copy(themeMode = mode) }
    }

    // Server config updates (require biometric before calling)

    fun updateServerConfig(
        consolidationWindowMin: Int? = null,
        interpretationDetail: String? = null,
        journalTone: String? = null,
        metadataRetentionDays: Int? = null,
        timezone: String? = null
    ) {
        viewModelScope.launch {
            _uiState.update { it.copy(isSavingServerConfig = true) }
            val request = UpdateConfigRequest(
                consolidationWindowMin = consolidationWindowMin,
                interpretationDetail = interpretationDetail,
                journalTone = journalTone,
                metadataRetentionDays = metadataRetentionDays,
                timezone = timezone
            )
            when (val result = settingsRepository.updateServerConfig(request)) {
                is ApiResult.Success -> {
                    timezoneProvider.update(result.data.timezone)
                    _uiState.update {
                        it.copy(serverConfig = result.data, isSavingServerConfig = false)
                    }
                }
                else -> {
                    _uiState.update {
                        it.copy(isSavingServerConfig = false, error = "Failed to save server config")
                    }
                }
            }
        }
    }

    // Device management

    fun generatePairingCode() {
        viewModelScope.launch {
            when (val result = authRepository.generatePairingCode()) {
                is ApiResult.Success -> {
                    _uiState.update { it.copy(generatedPairingCode = result.data.code) }
                }
                is ApiResult.Error -> {
                    _uiState.update { it.copy(error = result.message) }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update { it.copy(error = "Network error") }
                }
            }
        }
    }

    fun dismissPairingCode() {
        _uiState.update { it.copy(generatedPairingCode = null) }
    }

    fun logout() {
        authRepository.logout()
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }
}
