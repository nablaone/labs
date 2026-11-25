package szept.client

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import szept.crypto.Cbor
import szept.crypto.CryptoException
import szept.crypto.Envelope
import szept.crypto.Keys
import szept.crypto.Offer
import szept.crypto.Pairing
import szept.crypto.RecvChain
import szept.crypto.SendChain
import szept.crypto.Words
import szept.crypto.sha256
import java.io.IOException
import java.security.SecureRandom

enum class Connection { OFFLINE, CONNECTING, ONLINE }

/** A pairing in progress: our QR code. Lives only in memory (§3.2 step 4). */
class PairingSession internal constructor(internal val pairing: Pairing) {
    val qr: String = pairing.offer.encode()
}

/** Both QR codes scanned; waiting for the people to compare [words]. */
class ScannedPeer internal constructor(val session: PairingSession, val peer: Offer, internal val keys: Keys) {
    val words: List<String> get() = keys.confirm
}

/**
 * The client: owns the state, talks to the servers, and applies the protocol.
 * All state changes go through [edit], which persists before publishing.
 */
class Engine(
    private val store: StateStore,
    private val scope: CoroutineScope,
    private val api: Api = Api(),
    private val now: () -> Long = System::currentTimeMillis,
    private val rng: SecureRandom = SecureRandom(),
) {
    companion object {
        const val DEFAULT_RETENTION = 7 * 24 * 3600L
        const val ALIVE_MS = 72 * 3600_000L
        const val QUIET_MS = 14 * 24 * 3600_000L
        const val GONE_MS = 60 * 24 * 3600_000L
        /** How long a pending contact's hello is retried while the peer's mailbox doesn't exist yet. */
        const val PENDING_RETRY_MS = 24 * 3600_000L
        const val INVITE_PREFIX = "szept-invite:"

        /** Parses `szept-invite:<server>#<invite base64url>` (§8.2). */
        fun parseInvite(uri: String): Pair<String, ByteArray> {
            val s = uri.trim()
            if (!s.startsWith(INVITE_PREFIX)) throw IllegalArgumentException("not a szept invite")
            val hash = s.lastIndexOf('#')
            if (hash < 0) throw IllegalArgumentException("invite has no token")
            val inv = runCatching { Api.unb64(s.substring(hash + 1)) }.getOrNull()
            if (inv?.size != 16) throw IllegalArgumentException("bad invite token")
            return normServer(s.substring(INVITE_PREFIX.length, hash)) to inv
        }

        fun normServer(s: String): String {
            val u = s.trim().trimEnd('/')
            require(u.startsWith("https://") || u.startsWith("http://")) { "server must be an http(s) URL" }
            return u
        }
    }

    private val mutex = Mutex()
    private val _state = MutableStateFlow(store.load()?.let(Codec::decode) ?: AppState())
    val state: StateFlow<AppState> = _state

    private val _connection = MutableStateFlow(Connection.OFFLINE)
    val connection: StateFlow<Connection> = _connection

    private val _notices = MutableSharedFlow<String>(extraBufferCapacity = 16)
    /** Things the user should hear about, e.g. a contact that disappeared. */
    val notices: SharedFlow<String> = _notices

    private fun device() = _state.value.device ?: throw IllegalStateException("not set up")

    private suspend fun <T> edit(block: (AppState) -> Pair<AppState, T>): T = withContext(Dispatchers.IO) { editLocked(block) }

    private suspend fun <T> editLocked(block: (AppState) -> Pair<AppState, T>): T = mutex.withLock {
        val old = _state.value
        val (new, r) = block(old)
        if (new != old) {
            store.save(Codec.encode(new))
            _state.value = new
        }
        r
    }

    private suspend fun editContact(id: String, block: (Contact) -> Contact?) = edit { s ->
        val c = s.contact(id) ?: return@edit s to Unit
        val n = block(c)
        s.copy(contacts = if (n == null) s.contacts - c else s.contacts.map { if (it.id == id) n else it }) to Unit
    }

    private fun random(n: Int) = ByteArray(n).also(rng::nextBytes)

    // ---- setup (§2) ----

    /** Registers with the invite's server. Returns the kill code, to show once. */
    suspend fun register(server: String, invite: ByteArray, name: String): String {
        check(_state.value.device == null) { "already set up" }
        val token = random(32)
        val kill = Words.newKillCode(rng)
        api.register(normServer(server), invite, sha256(token), Words.killHash(kill))
        edit { it.copy(device = Device(normServer(server), token, name.trim())) to Unit }
        return kill
    }

    /** Accepts either an admin invite or a friend's pairing QR that carries an invite (§3.2). */
    suspend fun registerFrom(code: String, name: String): String {
        val c = code.trim()
        if (c.startsWith(Offer.QR_PREFIX)) {
            val o = Offer.decode(c)
            val inv = o.invite ?: throw IllegalArgumentException("this code has no invite; ask them to include one")
            return register(o.server, inv, name)
        }
        val (server, inv) = parseInvite(c)
        return register(server, inv, name)
    }

    suspend fun rename(name: String) {
        val n = name.trim()
        edit { s -> s.copy(device = s.device?.copy(name = n)) to Unit }
        for (c in _state.value.contacts.filter { it.active }) queue(c.id, payload("name", "name" to n))
        flushSoon()
    }

    /** Deletes this device and everything on the server, then forgets all state. */
    suspend fun wipe() {
        val d = device()
        for (c in _state.value.contacts) runCatching { postNow(c.id, payload("bye")) }
        runCatching { api.deleteDevice(d) }
        edit { AppState() to Unit }
    }

    /** A single-use invite to our server (§8.2). */
    suspend fun createInvite(): ByteArray = api.createInvite(device())

    // ---- pairing (§3) ----

    suspend fun startPairing(includeInvite: Boolean): PairingSession {
        val d = device()
        val inv = if (includeInvite) api.createInvite(d) else null
        return PairingSession(Pairing.create(d.server, d.name.ifEmpty { null }, inv, rng))
    }

    fun scan(session: PairingSession, qr: String): ScannedPeer {
        val peer = Offer.decode(qr.trim())
        return ScannedPeer(session, peer, session.pairing.complete(peer))
    }

    /** The words matched: create our mailbox, store the contact, send hello. */
    suspend fun confirm(p: ScannedPeer): String {
        val d = device()
        val k = p.keys
        api.putMailbox(d, k.recvMbox, sha256(k.recvToken))
        p.session.pairing.destroy()
        val t = now()
        val c = Contact(
            id = k.recvMbox.hex(),
            name = p.peer.name ?: "?",
            peerServer = normServer(p.peer.server),
            sendMbox = k.sendMbox, sendToken = k.sendToken, recvMbox = k.recvMbox,
            send = ChainState(k.sendCk, 0), recv = ChainState(k.recvCk, 0),
            active = false, created = t, lastIn = 0, lastOut = 0,
            retention = DEFAULT_RETENTION, messages = emptyList(), outbox = emptyList(), unread = 0,
        )
        edit { s -> s.copy(contacts = s.contacts + c) to Unit }
        queue(c.id, payload("hello", "name" to d.name, "ret" to DEFAULT_RETENTION))
        flushSoon()
        return c.id
    }

    // ---- sending ----

    private fun payload(t: String, vararg fields: Pair<String, Any?>): Map<String, Any?> =
        mapOf("t" to t, "id" to random(16), "ts" to now()) + fields

    private fun seal(c: Contact, p: Map<String, Any?>): Pair<Contact, ByteArray> {
        val chain = SendChain(c.send.ck, c.send.n)
        val env = Envelope.sealPairwise(chain, c.sendMbox, Cbor.encode(p))
        return c.copy(send = c.send.copy(ck = chain.ck, n = chain.n), lastOut = now()) to env
    }

    /** Seals now (advancing the chain) and appends to the outbox; [show] adds it to the chat. */
    private suspend fun queue(contactId: String, p: Map<String, Any?>, show: String? = null) = editContact(contactId) { c ->
        val (c2, env) = seal(c, p)
        val mid = (p["id"] as ByteArray).hex()
        val msgs = if (show != null) c2.messages + ChatMessage(mid, true, show, now(), MsgStatus.QUEUED) else c2.messages
        c2.copy(outbox = c2.outbox + Outgoing(env, if (show != null) mid else null, now()), messages = msgs)
    }

    /** Best effort, skipping the outbox; for bye. */
    private suspend fun postNow(contactId: String, p: Map<String, Any?>) {
        // Seal under the lock: a snapshot could reuse a counter another message just took.
        var sealed: Pair<Contact, ByteArray>? = null
        editContact(contactId) { c -> seal(c, p).also { sealed = c to it.second }.first }
        val (c, env) = sealed ?: return
        api.send(c.peerServer, c.sendMbox, c.sendToken, env)
    }

    suspend fun sendText(contactId: String, body: String) {
        val b = body.trim()
        if (b.isEmpty()) return
        queue(contactId, payload("text", "body" to b), show = b)
        flushSoon()
    }

    suspend fun setRetention(contactId: String, seconds: Long) {
        editContact(contactId) { it.copy(retention = seconds) }
        queue(contactId, payload("ret", "sec" to seconds))
        flushSoon()
    }

    /** The chat is on screen: clear the badge and send read receipts. */
    suspend fun markRead(contactId: String) {
        var ids = emptyList<String>()
        editContact(contactId) { c ->
            ids = c.messages.filter { !it.out && it.status == MsgStatus.RECEIVED }.map { it.id }
            if (ids.isEmpty() && c.unread == 0) c
            else c.copy(unread = 0, messages = c.messages.map { if (it.id in ids && !it.out) it.copy(status = MsgStatus.SEEN) else it })
        }
        if (ids.isNotEmpty()) {
            queue(contactId, payload("rcpt", "ids" to ids.map(::unhex), "k" to "r"))
            flushSoon()
        }
    }

    suspend fun deleteContact(contactId: String) {
        val c = _state.value.contact(contactId) ?: return
        runCatching { postNow(c.id, payload("bye")) }
        editContact(contactId) { null }
        runCatching { api.deleteMailbox(device(), c.recvMbox) }
    }

    private val flushLock = Mutex()
    private val flushAgain = java.util.concurrent.atomic.AtomicBoolean(false)

    private fun flushSoon() {
        scope.launch { flush() }
    }

    /** Posts queued envelopes in order, per contact. Stops a contact at the first transient failure. */
    suspend fun flush() {
        if (!flushLock.tryLock()) {
            flushAgain.set(true)
            return
        }
        try {
            do {
                flushAgain.set(false)
                for (id in _state.value.contacts.filter { it.outbox.isNotEmpty() }.map { it.id }) flushContact(id)
            } while (flushAgain.get())
        } finally {
            flushLock.unlock()
        }
    }

    private suspend fun flushContact(id: String) {
        while (true) {
            val c = _state.value.contact(id) ?: return
            val item = c.outbox.firstOrNull() ?: return
            try {
                api.send(c.peerServer, c.sendMbox, c.sendToken, item.envelope)
            } catch (e: ApiException) {
                when {
                    e.code == 404 && !c.active && now() - item.queued < PENDING_RETRY_MS -> return // peer hasn't confirmed yet
                    e.code == 404 -> { // §9: the contact is gone
                        editContact(id) { null }
                        runCatching { api.deleteMailbox(device(), c.recvMbox) }
                        _notices.tryEmit("${c.name} is gone (deleted or wiped)")
                        return
                    }
                    e.code == 403 || e.code == 413 || e.code == 400 -> Unit // will never succeed; drop it
                    else -> return // 429, 5xx: try later
                }
            } catch (e: IOException) {
                return
            }
            editContact(id) { cc ->
                cc.copy(
                    outbox = cc.outbox.filterNot { it === item },
                    messages = cc.messages.map { if (it.id == item.msgId && it.status == MsgStatus.QUEUED) it.copy(status = MsgStatus.SENT) else it },
                )
            }
        }
    }

    // ---- receiving ----

    /** Decrypts and applies one delivered message. Returns true when it can be acknowledged. */
    suspend fun receive(m: WireMsg): Boolean {
        var reply: Pair<String, Map<String, Any?>>? = null
        var notice: String? = null
        var gone: Contact? = null
        edit { s ->
            val c = s.contacts.firstOrNull { it.recvMbox.contentEquals(m.mailbox) } ?: return@edit s to Unit
            val chain = RecvChain(c.recv.ck, c.recv.n, HashMap(c.recv.skipped))
            val p = try {
                Cbor.decodeMap(Envelope.openPairwise(chain, c.recvMbox, m.body))
            } catch (e: CryptoException) {
                return@edit s to Unit // forged, replayed or garbage: drop it
            }
            val t = now()
            val skippedAt = chain.skipped.keys.associateWith { c.recv.skippedAt[it] ?: t }
            var n = c.copy(recv = ChainState(chain.ck, chain.n, HashMap(chain.skipped), skippedAt), lastIn = t, active = true)
            val mid = (p["id"] as? ByteArray)?.hex() ?: ""
            when (p["t"]) {
                "hello" -> {
                    (p["name"] as? String)?.takeIf { it.isNotBlank() }?.let { n = n.copy(name = it.take(64)) }
                    (p["ret"] as? Long)?.takeIf { it > 0 }?.let { n = n.copy(retention = it) }
                }
                "text" -> {
                    val body = p["body"] as? String ?: ""
                    if (n.messages.none { it.id == mid && !it.out }) {
                        n = n.copy(messages = n.messages + ChatMessage(mid, false, body, t, MsgStatus.RECEIVED), unread = n.unread + 1)
                        reply = c.id to payload("rcpt", "ids" to listOf(unhex(mid)), "k" to "d")
                    }
                }
                "rcpt" -> {
                    val ids = (p["ids"] as? List<*>)?.mapNotNull { (it as? ByteArray)?.hex() }?.toSet() ?: emptySet()
                    val to = if (p["k"] == "r") MsgStatus.READ else MsgStatus.DELIVERED
                    n = n.copy(messages = n.messages.map { if (it.out && it.id in ids && it.status < to) it.copy(status = to) else it })
                }
                "ret" -> (p["sec"] as? Long)?.takeIf { it > 0 }?.let { n = n.copy(retention = it) }
                "name" -> (p["name"] as? String)?.takeIf { it.isNotBlank() }?.let { n = n.copy(name = it.take(64)) }
                "bye" -> {
                    gone = c
                    notice = "${c.name} deleted the chat"
                    return@edit s.copy(contacts = s.contacts - c) to Unit
                }
                else -> Unit // alive, file and group messages (not yet supported), unknown types (§6)
            }
            s.copy(contacts = s.contacts.map { if (it.id == c.id) n else it }) to Unit
        }
        gone?.let { runCatching { api.deleteMailbox(device(), it.recvMbox) } }
        notice?.let { _notices.tryEmit(it) }
        reply?.let { (id, p) -> queue(id, p); flushSoon() }
        return true
    }

    /** Polling fallback (§8.5), e.g. from a background worker. */
    suspend fun poll() {
        val d = _state.value.device ?: return
        while (true) {
            val msgs = api.inbox(d)
            if (msgs.isEmpty()) return
            val ok = msgs.filter { receive(it) }.map { it.id }
            api.ack(d, ok)
            if (msgs.size < 100) return
        }
    }

    // ---- connection ----

    private var runJob: Job? = null

    /** Keeps the WebSocket open and runs housekeeping until [stop]. Idempotent. */
    @Synchronized
    fun start() {
        if (runJob?.isActive == true) return
        runJob = scope.launch {
            launch {
                var lastHousekeeping = 0L
                while (isActive) {
                    if (now() - lastHousekeeping >= 30_000) {
                        runCatching { housekeeping() }
                        lastHousekeeping = now()
                    }
                    runCatching { flush() }
                    // A pending hello is waiting for the peer to tap "Words match"; retry it quickly.
                    val pending = _state.value.contacts.any { !it.active && it.outbox.isNotEmpty() }
                    delay(if (pending) 3_000 else 30_000)
                }
            }
            var backoff = 1_000L
            while (isActive) {
                val d = _state.value.device
                if (d == null) {
                    delay(1_000); continue
                }
                _connection.value = Connection.CONNECTING
                val inbox = Channel<WireMsg>(Channel.UNLIMITED)
                val sock = api.connect(
                    d,
                    onOpen = {
                        _connection.value = Connection.ONLINE
                        backoff = 1_000L
                        scope.launch { runCatching { flush() } }
                    },
                    onMsg = { inbox.trySend(it) },
                    onClosed = { inbox.close() },
                )
                try {
                    for (m in inbox) {
                        if (receive(m)) sock.ack(m.id)
                    }
                } finally {
                    sock.close()
                }
                _connection.value = Connection.OFFLINE
                delay(backoff)
                backoff = minOf(backoff * 2, 60_000L)
            }
        }
    }

    @Synchronized
    fun stop() {
        runJob?.cancel()
        runJob = null
        _connection.value = Connection.OFFLINE
    }

    /** Retention, alive messages and the contact lifecycle (§6, §9). */
    suspend fun housekeeping() {
        val t = now()
        val alive = mutableListOf<String>()
        val dead = mutableListOf<Contact>()
        edit { s ->
            s.copy(
                contacts = s.contacts.mapNotNull { c ->
                    if (c.active && t - maxOf(c.lastIn, c.created) > GONE_MS) {
                        dead += c; return@mapNotNull null
                    }
                    val cutoff = t - c.retention * 1000
                    if (c.active && t - maxOf(c.lastOut, c.created) > ALIVE_MS) alive += c.id
                    val keep = c.recv.skippedAt.filterValues { it >= cutoff }.keys
                    c.copy(
                        messages = c.messages.filter { it.ts >= cutoff || it.status == MsgStatus.QUEUED },
                        recv = c.recv.copy(skipped = c.recv.skipped.filterKeys { it in keep }, skippedAt = c.recv.skippedAt.filterKeys { it in keep }),
                    )
                },
            ) to Unit
        }
        for (c in dead) {
            runCatching { api.deleteMailbox(device(), c.recvMbox) }
            _notices.tryEmit("${c.name} hasn't been seen for 60 days and was removed")
        }
        for (id in alive) queue(id, payload("alive"))
    }
}
