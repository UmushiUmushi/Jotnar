package com.jotnar.capture

import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ForegroundAppDetector @Inject constructor() {
    fun getCurrentForegroundApp(): String? {
        return JotnarAccessibilityService.currentForegroundApp.value
    }
}
