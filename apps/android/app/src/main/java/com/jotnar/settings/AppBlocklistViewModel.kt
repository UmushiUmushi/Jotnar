package com.jotnar.settings

import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import androidx.lifecycle.ViewModel
import com.jotnar.capture.AppBlocklist
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import javax.inject.Inject

data class BlocklistUiState(
    val allApps: List<InstalledApp> = emptyList(),
    val filteredApps: List<InstalledApp> = emptyList(),
    val searchQuery: String = "",
    val blockedCategories: Set<String> = emptySet(),
    val isLoading: Boolean = true
)

@HiltViewModel
class AppBlocklistViewModel @Inject constructor(
    private val devicePreferences: DevicePreferences
) : ViewModel() {

    private val _uiState = MutableStateFlow(
        BlocklistUiState(
            blockedCategories = devicePreferences.blockedCategories
        )
    )
    val uiState: StateFlow<BlocklistUiState> = _uiState.asStateFlow()

    fun loadInstalledApps(packageManager: PackageManager) {
        val blockedApps = devicePreferences.blockedApps
        val blockedCategories = devicePreferences.blockedCategories

        @Suppress("DEPRECATION")
        val apps = packageManager.getInstalledApplications(PackageManager.GET_META_DATA)
            .filter { it.flags and ApplicationInfo.FLAG_SYSTEM == 0 }
            .map { info ->
                val category = AppBlocklist.detectCategory(info)
                val isCategoryBlocked = category != null && category in blockedCategories
                InstalledApp(
                    packageName = info.packageName,
                    label = packageManager.getApplicationLabel(info).toString(),
                    isBlocked = info.packageName in blockedApps || isCategoryBlocked,
                    category = category,
                    isCategoryBlocked = isCategoryBlocked
                )
            }
            .sortedWith(compareByDescending<InstalledApp> { it.isBlocked }.thenBy { it.label })

        _uiState.update {
            it.copy(
                allApps = apps,
                filteredApps = apps,
                isLoading = false
            )
        }
    }

    fun updateSearch(query: String) {
        _uiState.update { state ->
            val filtered = if (query.isBlank()) {
                state.allApps
            } else {
                state.allApps.filter {
                    it.label.contains(query, ignoreCase = true) ||
                        it.packageName.contains(query, ignoreCase = true)
                }
            }
            state.copy(searchQuery = query, filteredApps = filtered)
        }
    }

    fun toggleApp(packageName: String) {
        val current = devicePreferences.blockedApps.toMutableSet()
        if (packageName in current) {
            current.remove(packageName)
        } else {
            current.add(packageName)
        }
        devicePreferences.blockedApps = current

        _uiState.update { state ->
            val updatedApps = state.allApps.map {
                if (it.packageName == packageName) {
                    val inExplicitList = packageName in current
                    it.copy(isBlocked = inExplicitList || it.isCategoryBlocked)
                } else {
                    it
                }
            }
            val filtered = applySearch(updatedApps, state.searchQuery)
            state.copy(allApps = updatedApps, filteredApps = filtered)
        }
    }

    fun toggleCategory(category: String) {
        val current = devicePreferences.blockedCategories.toMutableSet()
        val enabling = category !in current
        if (enabling) {
            current.add(category)
        } else {
            current.remove(category)
        }
        devicePreferences.blockedCategories = current

        // Update blocked state for apps in this category
        _uiState.update { state ->
            val blockedApps = devicePreferences.blockedApps
            val updatedApps = state.allApps.map { app ->
                if (app.category == category) {
                    app.copy(
                        isCategoryBlocked = enabling,
                        isBlocked = app.packageName in blockedApps || enabling
                    )
                } else {
                    app
                }
            }
            val filtered = applySearch(updatedApps, state.searchQuery)
            state.copy(
                allApps = updatedApps,
                filteredApps = filtered,
                blockedCategories = current
            )
        }
    }

    private fun applySearch(apps: List<InstalledApp>, query: String): List<InstalledApp> {
        if (query.isBlank()) return apps
        return apps.filter {
            it.label.contains(query, ignoreCase = true) ||
                it.packageName.contains(query, ignoreCase = true)
        }
    }
}
