package szept.app

import android.app.Application
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import szept.client.Engine

class SzeptApp : Application() {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val engine: Engine by lazy { Engine(KeystoreStore(this), scope) }
}
