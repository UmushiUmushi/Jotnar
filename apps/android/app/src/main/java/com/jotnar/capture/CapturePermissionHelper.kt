package com.jotnar.capture

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.media.projection.MediaProjectionManager
import androidx.activity.result.ActivityResult
import androidx.activity.result.ActivityResultLauncher

class CapturePermissionHelper(
    private val launcher: ActivityResultLauncher<Intent>
) {
    private var onResult: ((resultCode: Int, data: Intent?) -> Unit)? = null

    fun requestPermission(activity: Activity, onResult: (resultCode: Int, data: Intent?) -> Unit) {
        this.onResult = onResult
        val projectionManager = activity.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        launcher.launch(projectionManager.createScreenCaptureIntent())
    }

    fun handleResult(result: ActivityResult) {
        onResult?.invoke(result.resultCode, result.data)
        onResult = null
    }
}
