package szept.client

/** Delivery state of a chat message. */
enum class MsgStatus { QUEUED, SENT, DELIVERED, READ, RECEIVED, SEEN }

data class ChatMessage(
    /** Hex of the 16-byte message ID from the payload (§6). */
    val id: String,
    val out: Boolean,
    val body: String,
    /** Local clock: send time for outgoing, receive time for incoming. */
    val ts: Long,
    val status: MsgStatus,
)

/** Hash-ratchet state; skipped keys carry the time they were derived, for retention. */
data class ChainState(
    val ck: ByteArray,
    val n: Long,
    val skipped: Map<Long, ByteArray> = emptyMap(),
    val skippedAt: Map<Long, Long> = emptyMap(),
)

/** A sealed envelope waiting to be posted. Resent byte for byte, so the ratchet stays in step. */
data class Outgoing(val envelope: ByteArray, val msgId: String?, val queued: Long)

data class Contact(
    /** Hex of our receive mailbox; unique per pairing. */
    val id: String,
    val name: String,
    val peerServer: String,
    val sendMbox: ByteArray,
    val sendToken: ByteArray,
    val recvMbox: ByteArray,
    val send: ChainState,
    val recv: ChainState,
    /** False until the peer's hello arrives (§3.2 step 5). */
    val active: Boolean,
    val created: Long,
    val lastIn: Long,
    val lastOut: Long,
    /** Retention in seconds (§6). */
    val retention: Long,
    val messages: List<ChatMessage>,
    val outbox: List<Outgoing>,
    val unread: Int,
) {
    /** "not seen for 14 days" (§9). */
    fun quiet(now: Long) = active && now - maxOf(lastIn, created) > Engine.QUIET_MS
}

data class Device(val server: String, val token: ByteArray, val name: String)

data class AppState(val device: Device? = null, val contacts: List<Contact> = emptyList()) {
    fun contact(id: String) = contacts.firstOrNull { it.id == id }
}

/** Where the engine keeps its state. On Android this is an encrypted file. */
interface StateStore {
    fun load(): ByteArray?
    fun save(data: ByteArray)
}

class MemoryStore : StateStore {
    @Volatile private var data: ByteArray? = null
    override fun load() = data
    override fun save(data: ByteArray) { this.data = data }
}

internal fun ByteArray.hex() = joinToString("") { "%02x".format(it) }
internal fun unhex(s: String) = ByteArray(s.length / 2) { s.substring(2 * it, 2 * it + 2).toInt(16).toByte() }
