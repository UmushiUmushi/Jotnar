package com.jotnar.auth

import android.content.SharedPreferences
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ServerPreferences @Inject constructor(
    private val prefs: SharedPreferences
) {
    companion object {
        private const val KEY_SERVER_ADDRESS = "server_address"
    }

    var serverAddress: String?
        get() = prefs.getString(KEY_SERVER_ADDRESS, null)
        set(value) {
            prefs.edit().putString(KEY_SERVER_ADDRESS, value).apply()
        }
}
