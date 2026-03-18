package com.jotnar.capture

enum class CaptureState {
    Idle,
    Capturing,
    PausedBlockedApp,
    PausedBatterySaver,
    PausedLowBattery,
    PausedManual,
    Stopped
}
