package com.jotnar.settings

import java.time.ZoneId
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class TimezoneProvider @Inject constructor() {
    @Volatile
    var zoneId: ZoneId = ZoneId.of("UTC")
        private set

    fun update(timezoneId: String) {
        zoneId = try {
            ZoneId.of(timezoneId)
        } catch (_: Exception) {
            ZoneId.of("UTC")
        }
    }
}
