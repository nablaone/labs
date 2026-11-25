package szept.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import android.os.Build
import szept.client.Engine

class MainActivity : ComponentActivity() {
    private val engine get() = (application as SzeptApp).engine

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { SzeptTheme { Root(engine) } }
    }

    // Messages arrive over the WebSocket while the app is in the foreground.
    // Push (UnifiedPush) is the next step.
    override fun onStart() {
        super.onStart()
        engine.start()
    }

    override fun onStop() {
        super.onStop()
        engine.stop()
    }
}

@Composable
fun SzeptTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val ctx = LocalContext.current
    val colors = when {
        Build.VERSION.SDK_INT >= 31 -> if (dark) dynamicDarkColorScheme(ctx) else dynamicLightColorScheme(ctx)
        dark -> darkColorScheme()
        else -> lightColorScheme()
    }
    MaterialTheme(colorScheme = colors, content = content)
}

sealed interface Screen {
    data object Chats : Screen
    data class Chat(val id: String) : Screen
    /** [peerQr]: a friend's code already scanned during setup. */
    data class Pair(val peerQr: String? = null) : Screen
    data object Settings : Screen
}

@Composable
fun Root(engine: Engine) {
    val state by engine.state.collectAsState()
    var killCode by rememberSaveable { mutableStateOf<String?>(null) }
    var screen by remember { mutableStateOf<Screen>(Screen.Chats) }

    if (state.device == null) {
        SetupScreen(engine) { kill, peerQr ->
            killCode = kill
            screen = if (peerQr != null) Screen.Pair(peerQr) else Screen.Chats
        }
        return
    }
    killCode?.let {
        KillCodeScreen(it) { killCode = null }
        return
    }

    BackHandler(enabled = screen != Screen.Chats) { screen = Screen.Chats }
    when (val s = screen) {
        Screen.Chats -> ChatsScreen(engine, state, open = { screen = Screen.Chat(it) }, pair = { screen = Screen.Pair() }, settings = { screen = Screen.Settings })
        is Screen.Chat -> ChatScreen(engine, state, s.id, back = { screen = Screen.Chats })
        is Screen.Pair -> PairScreen(engine, s.peerQr, back = { screen = Screen.Chats }, done = { screen = Screen.Chat(it) })
        Screen.Settings -> SettingsScreen(engine, state, back = { screen = Screen.Chats })
    }
}
