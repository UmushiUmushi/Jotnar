package com.jotnar.settings

import android.content.SharedPreferences
import com.jotnar.ui.theme.ThemeMode
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class DevicePreferences @Inject constructor(
    private val prefs: SharedPreferences
) {
    // Theme
    private val _themeModeFlow = MutableStateFlow(readThemeMode())
    val themeModeFlow: StateFlow<ThemeMode> = _themeModeFlow.asStateFlow()

    var themeMode: ThemeMode
        get() = _themeModeFlow.value
        set(value) {
            prefs.edit().putString(KEY_THEME_MODE, value.name).apply()
            _themeModeFlow.value = value
        }

    private fun readThemeMode(): ThemeMode = try {
        ThemeMode.valueOf(prefs.getString(KEY_THEME_MODE, ThemeMode.MaterialYou.name)!!)
    } catch (_: Exception) {
        ThemeMode.MaterialYou
    }

    // Capture
    var captureEnabled: Boolean
        get() = prefs.getBoolean(KEY_CAPTURE_ENABLED, true)
        set(value) = prefs.edit().putBoolean(KEY_CAPTURE_ENABLED, value).apply()

    var captureIntervalSec: Int
        get() = prefs.getInt(KEY_CAPTURE_INTERVAL, 10)
        set(value) = prefs.edit().putInt(KEY_CAPTURE_INTERVAL, value).apply()

    var pauseOnBatterySaver: Boolean
        get() = prefs.getBoolean(KEY_PAUSE_BATTERY_SAVER, true)
        set(value) = prefs.edit().putBoolean(KEY_PAUSE_BATTERY_SAVER, value).apply()

    var pauseOnLowBattery: Boolean
        get() = prefs.getBoolean(KEY_PAUSE_LOW_BATTERY, false)
        set(value) = prefs.edit().putBoolean(KEY_PAUSE_LOW_BATTERY, value).apply()

    var lowBatteryThreshold: Int
        get() = prefs.getInt(KEY_LOW_BATTERY_THRESHOLD, 15)
        set(value) = prefs.edit().putInt(KEY_LOW_BATTERY_THRESHOLD, value).apply()

    var wifiOnlyUpload: Boolean
        get() = prefs.getBoolean(KEY_WIFI_ONLY, true)
        set(value) = prefs.edit().putBoolean(KEY_WIFI_ONLY, value).apply()

    // Privacy
    var blockedApps: Set<String>
        get() = prefs.getStringSet(KEY_BLOCKED_APPS, emptySet()) ?: emptySet()
        set(value) = prefs.edit().putStringSet(KEY_BLOCKED_APPS, value).apply()

    var blockedCategories: Set<String>
        get() = prefs.getStringSet(KEY_BLOCKED_CATEGORIES, setOf("finance", "health", "auth"))
            ?: setOf("finance", "health", "auth")
        set(value) = prefs.edit().putStringSet(KEY_BLOCKED_CATEGORIES, value).apply()

    var notificationStyle: String
        get() = prefs.getString(KEY_NOTIFICATION_STYLE, "persistent") ?: "persistent"
        set(value) = prefs.edit().putString(KEY_NOTIFICATION_STYLE, value).apply()

    var captureReminderEnabled: Boolean
        get() = prefs.getBoolean(KEY_CAPTURE_REMINDER, true)
        set(value) = prefs.edit().putBoolean(KEY_CAPTURE_REMINDER, value).apply()

    // Connection
    var uploadBatchSize: Int
        get() = prefs.getInt(KEY_UPLOAD_BATCH_SIZE, 10)
        set(value) = prefs.edit().putInt(KEY_UPLOAD_BATCH_SIZE, value.coerceIn(1, 50)).apply()

    var uploadRetrySec: Int
        get() = prefs.getInt(KEY_UPLOAD_RETRY_SEC, 60)
        set(value) = prefs.edit().putInt(KEY_UPLOAD_RETRY_SEC, value).apply()

    var serverTimeoutSec: Int
        get() = prefs.getInt(KEY_SERVER_TIMEOUT, 30)
        set(value) = prefs.edit().putInt(KEY_SERVER_TIMEOUT, value).apply()

    // Capture service state (not a user setting, used to restore after reboot)
    var captureWasRunning: Boolean
        get() = prefs.getBoolean(KEY_CAPTURE_WAS_RUNNING, false)
        set(value) = prefs.edit().putBoolean(KEY_CAPTURE_WAS_RUNNING, value).apply()

    companion object {
        private const val KEY_THEME_MODE = "theme_mode"
        private const val KEY_CAPTURE_ENABLED = "capture_enabled"
        private const val KEY_CAPTURE_INTERVAL = "capture_interval_sec"
        private const val KEY_PAUSE_BATTERY_SAVER = "pause_on_battery_saver"
        private const val KEY_PAUSE_LOW_BATTERY = "pause_on_low_battery"
        private const val KEY_LOW_BATTERY_THRESHOLD = "low_battery_threshold"
        private const val KEY_WIFI_ONLY = "wifi_only_upload"
        private const val KEY_BLOCKED_APPS = "blocked_apps"
        private const val KEY_BLOCKED_CATEGORIES = "blocked_categories"
        private const val KEY_NOTIFICATION_STYLE = "notification_style"
        private const val KEY_CAPTURE_REMINDER = "capture_reminder_enabled"
        private const val KEY_UPLOAD_BATCH_SIZE = "upload_batch_size"
        private const val KEY_UPLOAD_RETRY_SEC = "upload_retry_sec"
        private const val KEY_SERVER_TIMEOUT = "server_timeout_sec"
        private const val KEY_CAPTURE_WAS_RUNNING = "capture_was_running"
    }
}
