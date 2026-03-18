package com.jotnar.capture

import android.content.pm.ApplicationInfo
import com.jotnar.settings.DevicePreferences
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AppBlocklist @Inject constructor(
    private val devicePreferences: DevicePreferences
) {
    fun isBlocked(packageName: String): Boolean {
        if (packageName in devicePreferences.blockedApps) return true

        // Check blocked categories by known package prefixes
        val blockedCategories = devicePreferences.blockedCategories
        for (category in blockedCategories) {
            val prefixes = CATEGORY_PACKAGE_PREFIXES[category] ?: continue
            if (prefixes.any { packageName.startsWith(it) }) return true
        }

        return false
    }

    companion object {
        // Known package prefixes for category detection
        private val CATEGORY_PACKAGE_PREFIXES = mapOf(
            "finance" to listOf(
                "com.google.android.apps.walletnfcrel",
                "com.paypal", "com.venmo", "com.squareup.cash",
                "com.coinbase", "com.robinhood", "com.wealthsimple",
                "com.eqbank", "com.td", "com.rbc", "com.bmo",
                "com.cibc", "com.scotiabank", "com.interac",
                "com.chase", "com.bankofamerica", "com.wellsfargo"
            ),
            "health" to listOf(
                "com.google.android.apps.fitness",
                "com.myfitnesspal", "com.fitbit",
                "com.samsung.android.apps.shealth"
            ),
            "auth" to listOf(
                "com.google.android.apps.authenticator2",
                "com.authy.authy", "org.keepassxc",
                "com.onepassword", "com.lastpass",
                "com.bitwarden", "com.dashlane"
            )
        )

        /**
         * Detect which blocked category an app belongs to, if any.
         * Uses both Android's built-in category and known package prefixes.
         */
        fun detectCategory(info: ApplicationInfo): String? {
            // Check known package prefixes first (more precise)
            for ((category, prefixes) in CATEGORY_PACKAGE_PREFIXES) {
                if (prefixes.any { info.packageName.startsWith(it) }) {
                    return category
                }
            }
            return null
        }
    }
}
