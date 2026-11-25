@file:OptIn(ExperimentalMaterial3Api::class)

package szept.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Badge
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import szept.client.ApiException
import szept.client.Api
import szept.client.AppState
import szept.client.Connection
import szept.client.Engine
import szept.client.MsgStatus
import szept.client.PairingSession
import szept.client.ScannedPeer
import szept.crypto.Offer
import java.io.IOException
import java.text.DateFormat
import java.util.Date

// ---- helpers ----

private fun friendly(e: Throwable): String = when (e) {
    is ApiException -> when (e.code) {
        403 -> "The server refused: the invite is wrong, used or expired."
        409 -> "That already exists on the server."
        429 -> "Too many attempts; wait a bit."
        else -> "Server error ${e.code}: ${e.message}"
    }
    is IOException -> "Can't reach the server: ${e.message}"
    else -> e.message ?: e.toString()
}

/** Runs [block], reporting failures through [onError]; [busy] is true meanwhile. */
private fun CoroutineScope.attempt(busy: (Boolean) -> Unit, onError: (String?) -> Unit, block: suspend () -> Unit) = launch {
    busy(true)
    onError(null)
    try {
        block()
    } catch (e: Exception) {
        onError(friendly(e))
    } finally {
        busy(false)
    }
}

@Composable
private fun Bar(title: String, back: (() -> Unit)? = null, actions: @Composable () -> Unit = {}) {
    TopAppBar(
        title = { Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        navigationIcon = { if (back != null) TextButton(onClick = back) { Text("←", fontSize = 22.sp) } },
        actions = { actions() },
    )
}

@Composable
private fun ErrorText(error: String?) {
    if (error != null) Text(error, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(vertical = 8.dp))
}

@Composable
private fun PasteField(label: String, busy: Boolean, onSubmit: (String) -> Unit) {
    var text by rememberSaveable { mutableStateOf("") }
    Row(verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(text, { text = it }, label = { Text(label) }, singleLine = true, modifier = Modifier.weight(1f))
        Spacer(Modifier.size(8.dp))
        OutlinedButton(onClick = { onSubmit(text) }, enabled = !busy && text.isNotBlank()) { Text("Use") }
    }
}

// ---- setup (§2) ----

@Composable
fun SetupScreen(engine: Engine, done: (killCode: String, peerQr: String?) -> Unit) {
    val scope = rememberCoroutineScope()
    var name by rememberSaveable { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    fun use(code: String) = scope.attempt({ busy = it }, { error = it }) {
        val kill = engine.registerFrom(code, name)
        done(kill, code.trim().takeIf { it.startsWith(Offer.QR_PREFIX) })
    }
    val scan = rememberScanner("Scan an invite or a friend's szept code", ::use)

    Scaffold(topBar = { Bar("szept") }) { pad ->
        Column(Modifier.padding(pad).padding(16.dp).fillMaxSize().verticalScroll(rememberScrollState())) {
            Text("A small messenger for friends and family. There are no accounts: you add people by meeting them and scanning each other's codes.")
            Spacer(Modifier.height(16.dp))
            OutlinedTextField(name, { name = it.take(64) }, label = { Text("Your name, as contacts will see it") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(16.dp))
            Text("To start you need an invite from a server: either the server's own invite, or a friend's pairing code that includes one.")
            Spacer(Modifier.height(12.dp))
            Button(onClick = scan, enabled = !busy && name.isNotBlank(), modifier = Modifier.fillMaxWidth()) { Text("Scan code") }
            Spacer(Modifier.height(12.dp))
            PasteField("…or paste szept-invite: / szept1:", busy || name.isBlank(), ::use)
            ErrorText(error)
            if (busy) Text("Registering…")
        }
    }
}

@Composable
fun KillCodeScreen(code: String, done: () -> Unit) {
    Scaffold(topBar = { Bar("Kill code") }) { pad ->
        Column(Modifier.padding(pad).padding(16.dp).fillMaxSize().verticalScroll(rememberScrollState())) {
            Text("Write these six words down and keep them away from the phone. If the phone is lost or stolen, entering them on the server deletes this device and all its mailboxes.")
            Spacer(Modifier.height(24.dp))
            Surface(tonalElevation = 2.dp, shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(20.dp)) {
                    code.split(" ").forEachIndexed { i, w ->
                        Text("${i + 1}. $w", fontFamily = FontFamily.Monospace, fontSize = 22.sp, modifier = Modifier.padding(vertical = 4.dp))
                    }
                }
            }
            Spacer(Modifier.height(16.dp))
            Text("They are shown only this once.", fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(24.dp))
            Button(onClick = done, modifier = Modifier.fillMaxWidth()) { Text("I wrote them down (or I'll skip it)") }
        }
    }
}

// ---- chat list ----

@Composable
private fun ConnectionDot(engine: Engine) {
    val c by engine.connection.collectAsState()
    val (color, label) = when (c) {
        Connection.ONLINE -> Color(0xFF2E7D32) to "online"
        Connection.CONNECTING -> Color(0xFFF9A825) to "connecting"
        Connection.OFFLINE -> Color(0xFF9E9E9E) to "offline"
    }
    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(end = 8.dp)) {
        Box(Modifier.size(10.dp).padding(1.dp)) { Surface(shape = CircleShape, color = color, modifier = Modifier.fillMaxSize()) {} }
        Spacer(Modifier.size(6.dp))
        Text(label, style = MaterialTheme.typography.labelMedium)
    }
}

@Composable
private fun Notices(engine: Engine, host: SnackbarHostState) {
    LaunchedEffect(engine) { engine.notices.collect { host.showSnackbar(it) } }
}

@Composable
fun ChatsScreen(engine: Engine, state: AppState, open: (String) -> Unit, pair: () -> Unit, settings: () -> Unit) {
    val host = remember { SnackbarHostState() }
    Notices(engine, host)
    val now = System.currentTimeMillis()
    Scaffold(
        topBar = { Bar("szept") { ConnectionDot(engine); TextButton(onClick = settings) { Text("Settings") } } },
        floatingActionButton = { ExtendedFloatingActionButton(onClick = pair) { Text("Pair") } },
        snackbarHost = { SnackbarHost(host) },
    ) { pad ->
        if (state.contacts.isEmpty()) {
            Box(Modifier.padding(pad).fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
                Text("No contacts yet. Meet someone and tap Pair: you scan their code, they scan yours.", textAlign = TextAlign.Center)
            }
            return@Scaffold
        }
        val sorted = state.contacts.sortedByDescending { c -> c.messages.lastOrNull()?.ts ?: c.created }
        LazyColumn(Modifier.padding(pad).fillMaxSize()) {
            items(sorted, key = { it.id }) { c ->
                val last = c.messages.lastOrNull()
                val sub = when {
                    !c.active -> "waiting for them to confirm…"
                    c.quiet(now) -> "not seen for 14 days"
                    last != null -> (if (last.out) "you: " else "") + last.body
                    else -> "paired"
                }
                Row(
                    Modifier.fillMaxWidth().clickable { open(c.id) }.padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(c.name, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(sub, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    if (c.unread > 0) Badge { Text("${c.unread}") }
                }
                HorizontalDivider()
            }
        }
    }
}

// ---- pairing (§3) ----

@Composable
fun PairScreen(engine: Engine, initialPeerQr: String?, back: () -> Unit, done: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current
    var includeInvite by rememberSaveable { mutableStateOf(false) }
    var session by remember { mutableStateOf<PairingSession?>(null) }
    var peer by remember { mutableStateOf<ScannedPeer?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    // Fresh pairing material each time the screen opens or the invite option changes (§3.1).
    LaunchedEffect(includeInvite) {
        peer = null
        session = null
        try {
            val s = engine.startPairing(includeInvite)
            session = s
            if (initialPeerQr != null) peer = engine.scan(s, initialPeerQr)
        } catch (e: Exception) {
            error = friendly(e)
        }
    }

    fun scanned(qr: String) {
        val s = session ?: return
        error = null
        try {
            peer = engine.scan(s, qr)
        } catch (e: Exception) {
            error = "That isn't a szept pairing code (${e.message})."
        }
    }
    val scan = rememberScanner("Scan their szept code", ::scanned)

    Scaffold(topBar = { Bar("Pair", back) }) { pad ->
        Column(Modifier.padding(pad).padding(16.dp).fillMaxSize().verticalScroll(rememberScrollState())) {
            Text("Both phones show their code. You scan theirs, they scan yours.")
            Spacer(Modifier.height(12.dp))
            val s = session
            if (s == null) Text(if (error == null) "Preparing…" else "") else Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                // Smaller once scanned, so the words and buttons fit on screen.
                QrCode(s.qr, Modifier.widthIn(max = if (peer == null) 360.dp else 200.dp))
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Checkbox(includeInvite, { includeInvite = it }, enabled = peer == null)
                Text("Include an invite to my server, for someone who has no server yet")
            }
            Spacer(Modifier.height(8.dp))

            val p = peer
            if (p == null) {
                Button(onClick = scan, enabled = s != null, modifier = Modifier.fillMaxWidth()) { Text("Scan their code") }
                Spacer(Modifier.height(8.dp))
                Row {
                    TextButton(onClick = { s?.let { clipboard.setText(AnnotatedString(it.qr)) } }, enabled = s != null) { Text("Copy my code") }
                }
                Text("Pairing remotely (copying codes over a call or another chat) works, but anyone who sees both codes on that channel learns part of the secret. In person is better.", style = MaterialTheme.typography.bodySmall)
                PasteField("…or paste their szept1: code", s == null, ::scanned)
            } else {
                Text("Check that both phones show the same three words:", fontWeight = FontWeight.Bold)
                Surface(tonalElevation = 2.dp, shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp)) {
                    Column(Modifier.fillMaxWidth().padding(16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                        for (w in p.words) Text(w, fontSize = 26.sp, fontFamily = FontFamily.Monospace)
                    }
                }
                Text("Pairing with: ${p.peer.name ?: "(no name)"} on ${p.peer.server}")
                Text("Keep showing your code until they have scanned it too.", style = MaterialTheme.typography.bodySmall)
                Spacer(Modifier.height(12.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(onClick = { scope.attempt({ busy = it }, { error = it }) { done(engine.confirm(p)) } }, enabled = !busy) { Text("Words match") }
                    OutlinedButton(onClick = { peer = null }, enabled = !busy) { Text("They don't") }
                }
            }
            ErrorText(error)
        }
    }
}

// ---- one chat ----

private val timeFmt: DateFormat = DateFormat.getTimeInstance(DateFormat.SHORT)

private fun statusMark(s: MsgStatus) = when (s) {
    MsgStatus.QUEUED -> "…"
    MsgStatus.SENT -> "✓"
    MsgStatus.DELIVERED -> "✓✓"
    MsgStatus.READ -> "✓✓ read"
    else -> ""
}

@Composable
fun ChatScreen(engine: Engine, state: AppState, id: String, back: () -> Unit) {
    val c = state.contact(id)
    LaunchedEffect(c == null) { if (c == null) back() }
    if (c == null) return
    val scope = rememberCoroutineScope()
    val host = remember { SnackbarHostState() }
    Notices(engine, host)
    var text by rememberSaveable(id) { mutableStateOf("") }
    var menu by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf(false) }
    val list = rememberLazyListState()

    // On screen means read (§6 rcpt "r").
    LaunchedEffect(c.messages.size, c.unread) { engine.markRead(id) }
    LaunchedEffect(c.messages.size) { if (c.messages.isNotEmpty()) list.animateScrollToItem(c.messages.size - 1) }

    Scaffold(
        topBar = {
            Bar(c.name, back) {
                TextButton(onClick = { menu = true }) { Text("⋮", fontSize = 22.sp) }
                DropdownMenu(menu, { menu = false }) {
                    for ((label, sec) in listOf("Keep messages 1 day" to 86400L, "Keep messages 7 days" to 604800L, "Keep messages 30 days" to 2592000L)) {
                        DropdownMenuItem(
                            text = { Text(label + if (c.retention == sec) "  ✓" else "") },
                            onClick = { menu = false; scope.launch { runCatching { engine.setRetention(id, sec) } } },
                        )
                    }
                    DropdownMenuItem(text = { Text("Delete chat") }, onClick = { menu = false; confirmDelete = true })
                }
            }
        },
        snackbarHost = { SnackbarHost(host) },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize().imePadding()) {
            if (!c.active) {
                Text("Waiting for ${c.name} to confirm the pairing. You can already write; messages go out once they do.", style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(12.dp))
            }
            LazyColumn(Modifier.weight(1f).fillMaxWidth().padding(horizontal = 8.dp), state = list) {
                items(c.messages, key = { (if (it.out) "o" else "i") + it.id }) { m ->
                    Row(Modifier.fillMaxWidth().padding(vertical = 3.dp), horizontalArrangement = if (m.out) Arrangement.End else Arrangement.Start) {
                        Surface(
                            shape = RoundedCornerShape(14.dp),
                            color = if (m.out) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant,
                            modifier = Modifier.widthIn(max = 300.dp),
                        ) {
                            Column(Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                                Text(m.body)
                                Text(
                                    timeFmt.format(Date(m.ts)) + if (m.out) "  " + statusMark(m.status) else "",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.align(Alignment.End),
                                )
                            }
                        }
                    }
                }
            }
            Text(
                "Messages disappear after ${c.retention / 86400} day(s).",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 12.dp),
            )
            Row(Modifier.padding(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(text, { text = it }, modifier = Modifier.weight(1f), placeholder = { Text("Message") }, maxLines = 5)
                Spacer(Modifier.size(8.dp))
                Button(onClick = {
                    val b = text
                    text = ""
                    scope.launch { runCatching { engine.sendText(id, b) } }
                }, enabled = text.isNotBlank()) { Text("Send") }
            }
        }
    }

    if (confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete chat with ${c.name}?") },
            text = { Text("They are told, and the chat is deleted on both phones. To talk again you'd have to pair in person again.") },
            confirmButton = { TextButton(onClick = { confirmDelete = false; scope.launch { engine.deleteContact(id) } }) { Text("Delete") } },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
        )
    }
}

// ---- settings ----

@Composable
fun SettingsScreen(engine: Engine, state: AppState, back: () -> Unit) {
    val d = state.device ?: return
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current
    var name by rememberSaveable { mutableStateOf(d.name) }
    var invite by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var confirmWipe by remember { mutableStateOf(false) }

    Scaffold(topBar = { Bar("Settings", back) }) { pad ->
        Column(Modifier.padding(pad).padding(16.dp).fillMaxSize().verticalScroll(rememberScrollState())) {
            Text("Server", style = MaterialTheme.typography.titleSmall)
            Text(d.server, fontFamily = FontFamily.Monospace)
            Spacer(Modifier.height(20.dp))

            Text("Your name", style = MaterialTheme.typography.titleSmall)
            Row(verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(name, { name = it.take(64) }, singleLine = true, modifier = Modifier.weight(1f))
                Spacer(Modifier.size(8.dp))
                OutlinedButton(onClick = { scope.attempt({ busy = it }, { error = it }) { engine.rename(name) } }, enabled = !busy && name.isNotBlank() && name.trim() != d.name) { Text("Save") }
            }
            Spacer(Modifier.height(20.dp))

            Text("Invite someone to your server", style = MaterialTheme.typography.titleSmall)
            Text("Single use, valid for 7 days. Usually simpler: tick \"include an invite\" when pairing.", style = MaterialTheme.typography.bodySmall)
            val inv = invite
            if (inv == null) {
                OutlinedButton(onClick = {
                    scope.attempt({ busy = it }, { error = it }) {
                        invite = Engine.INVITE_PREFIX + d.server + "#" + Api.b64(engine.createInvite())
                    }
                }, enabled = !busy) { Text("Create invite") }
            } else {
                QrCode(inv, Modifier.widthIn(max = 300.dp).padding(vertical = 8.dp))
                TextButton(onClick = { clipboard.setText(AnnotatedString(inv)) }) { Text("Copy invite") }
            }
            Spacer(Modifier.height(20.dp))

            Text("This phone", style = MaterialTheme.typography.titleSmall)
            OutlinedButton(onClick = { confirmWipe = true }, enabled = !busy) { Text("Delete this device and all chats") }
            ErrorText(error)
        }
    }

    if (confirmWipe) {
        AlertDialog(
            onDismissRequest = { confirmWipe = false },
            title = { Text("Delete everything?") },
            text = { Text("Your contacts are told goodbye, your mailboxes are deleted from the server, and all chats are erased from this phone. There is no undo.") },
            confirmButton = { TextButton(onClick = { confirmWipe = false; scope.attempt({ busy = it }, { error = it }) { engine.wipe() } }) { Text("Delete") } },
            dismissButton = { TextButton(onClick = { confirmWipe = false }) { Text("Cancel") } },
        )
    }
}
