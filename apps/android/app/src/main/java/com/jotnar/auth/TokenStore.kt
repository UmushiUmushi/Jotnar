package com.jotnar.auth

import android.content.SharedPreferences
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class TokenStore @Inject constructor(
    private val prefs: SharedPreferences
) {
    companion object {
        private const val KEY_TOKEN = "device_token"
        private const val KEY_DEVICE_ID = "device_id"
    }

    val token: String?
        get() = prefs.getString(KEY_TOKEN, null)

    val deviceId: String?
        get() = prefs.getString(KEY_DEVICE_ID, null)

    val isAuthenticated: Boolean
        get() = !token.isNullOrBlank()

    fun save(token: String, deviceId: String) {
        prefs.edit()
            .putString(KEY_TOKEN, token)
            .putString(KEY_DEVICE_ID, deviceId)
            .apply()
    }

    fun clear() {
        prefs.edit()
            .remove(KEY_TOKEN)
            .remove(KEY_DEVICE_ID)
            .apply()
    }
}
