package com.jotnar.ui.components

import androidx.compose.material3.Button
import androidx.compose.material3.ButtonColors
import androidx.compose.material3.ButtonDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.fragment.app.FragmentActivity
import com.jotnar.auth.BiometricHelper
import kotlinx.coroutines.launch

@Composable
fun BiometricGateButton(
    onClick: () -> Unit,
    biometricHelper: BiometricHelper,
    title: String = "Authenticate",
    subtitle: String = "Confirm your identity to proceed",
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    colors: ButtonColors = ButtonDefaults.buttonColors(),
    content: @Composable () -> Unit
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    Button(
        onClick = {
            val activity = context as? FragmentActivity
            if (activity != null && biometricHelper.canAuthenticate(activity)) {
                scope.launch {
                    val authenticated = biometricHelper.authenticate(activity, title, subtitle)
                    if (authenticated) onClick()
                }
            } else {
                // No biometric available, proceed without gate
                onClick()
            }
        },
        modifier = modifier,
        enabled = enabled,
        colors = colors,
        content = { content() }
    )
}
