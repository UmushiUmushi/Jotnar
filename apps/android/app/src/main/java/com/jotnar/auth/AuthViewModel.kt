package com.jotnar.auth

import android.os.Build
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.jotnar.network.ApiResult
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class AuthUiState(
    val serverAddress: String = "",
    val pairingCode: String = "",
    val recoveryKey: String = "",
    val phase: AuthPhase = AuthPhase.ServerEntry,
    val serverStatus: ServerConnectionStatus = ServerConnectionStatus.Idle,
    val error: String? = null,
    val isLoading: Boolean = false,
    val recoveredPairingCode: String? = null
)

enum class AuthPhase {
    ServerEntry,
    PairingCode,
    Recovery,
    Success
}

enum class ServerConnectionStatus {
    Idle,
    Checking,
    Connected,
    ModelReady,
    Failed
}

@HiltViewModel
class AuthViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val biometricHelper: BiometricHelper
) : ViewModel() {

    private val _uiState = MutableStateFlow(AuthUiState())
    val uiState: StateFlow<AuthUiState> = _uiState.asStateFlow()

    fun updateServerAddress(address: String) {
        _uiState.update { it.copy(serverAddress = address, error = null) }
    }

    fun updatePairingCode(code: String) {
        if (code.length <= 6) {
            _uiState.update { it.copy(pairingCode = code.uppercase(), error = null) }
        }
    }

    fun updateRecoveryKey(key: String) {
        _uiState.update { it.copy(recoveryKey = key, error = null) }
    }

    fun checkServer() {
        viewModelScope.launch {
            _uiState.update { it.copy(serverStatus = ServerConnectionStatus.Checking, error = null) }
            authRepository.saveServerAddress(_uiState.value.serverAddress)
            when (val result = authRepository.checkServerStatus()) {
                is ApiResult.Success -> {
                    val status = if (result.data.modelAvailable) {
                        ServerConnectionStatus.ModelReady
                    } else {
                        ServerConnectionStatus.Connected
                    }
                    _uiState.update {
                        it.copy(
                            serverStatus = status,
                            phase = AuthPhase.PairingCode,
                            error = null
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(
                            serverStatus = ServerConnectionStatus.Failed,
                            error = "Server error: ${result.message}"
                        )
                    }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update {
                        it.copy(
                            serverStatus = ServerConnectionStatus.Failed,
                            error = "Cannot reach server. Check the address and try again."
                        )
                    }
                }
            }
        }
    }

    fun pair(activity: FragmentActivity) {
        val state = _uiState.value
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }

            val authenticated = biometricHelper.authenticate(
                activity = activity,
                title = "Confirm pairing",
                subtitle = "Verify your identity to pair this device"
            )
            if (!authenticated) {
                _uiState.update { it.copy(isLoading = false, error = "Authentication required to pair device") }
                return@launch
            }

            val deviceName = "${Build.MANUFACTURER} ${Build.MODEL}"
            when (val result = authRepository.pair(state.serverAddress, state.pairingCode, deviceName)) {
                is ApiResult.Success -> {
                    _uiState.update { it.copy(isLoading = false, phase = AuthPhase.Success) }
                }
                is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, error = result.message)
                    }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update {
                        it.copy(isLoading = false, error = "Network error. Check your connection.")
                    }
                }
            }
        }
    }

    fun recover() {
        val state = _uiState.value
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            when (val result = authRepository.recover(state.serverAddress, state.recoveryKey)) {
                is ApiResult.Success -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            recoveredPairingCode = result.data.pairingCode,
                            pairingCode = result.data.pairingCode,
                            phase = AuthPhase.PairingCode
                        )
                    }
                }
                is ApiResult.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, error = result.message)
                    }
                }
                is ApiResult.NetworkError -> {
                    _uiState.update {
                        it.copy(isLoading = false, error = "Network error. Check your connection.")
                    }
                }
            }
        }
    }

    fun navigateToRecovery() {
        _uiState.update { it.copy(phase = AuthPhase.Recovery, error = null) }
    }

    fun navigateBack() {
        _uiState.update {
            when (it.phase) {
                AuthPhase.PairingCode -> it.copy(phase = AuthPhase.ServerEntry, error = null)
                AuthPhase.Recovery -> it.copy(phase = AuthPhase.ServerEntry, error = null)
                else -> it
            }
        }
    }
}
