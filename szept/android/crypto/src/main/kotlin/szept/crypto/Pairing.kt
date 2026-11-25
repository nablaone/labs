package szept.crypto

import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import java.security.SecureRandom
import java.util.Base64

/** The QR payload (§3.1). */
class Offer(
    val e: ByteArray,
    val r: ByteArray,
    val server: String,
    val name: String? = null,
    val invite: ByteArray? = null,
) {
    fun encode(): String {
        val m = linkedMapOf<String, Any?>("v" to Const.VERSION.toLong(), "E" to e, "r" to r, "s" to server)
        if (!name.isNullOrEmpty()) m["n"] = name
        if (invite != null && invite.isNotEmpty()) m["i"] = invite
        return QR_PREFIX + Base64.getUrlEncoder().withoutPadding().encodeToString(Cbor.encode(m))
    }

    companion object {
        const val QR_PREFIX = "szept1:"

        fun decode(qr: String): Offer {
            if (!qr.startsWith(QR_PREFIX)) throw CryptoException("not a szept v1 QR code")
            val raw = try {
                Base64.getUrlDecoder().decode(qr.substring(QR_PREFIX.length))
            } catch (e: IllegalArgumentException) {
                throw CryptoException("bad base64")
            }
            val m = Cbor.decodeMap(raw)
            if (m["v"] != Const.VERSION.toLong()) throw CryptoException("unsupported version")
            val e = m["E"] as? ByteArray
            val r = m["r"] as? ByteArray
            if (e?.size != 32 || r?.size != 32) throw CryptoException("bad key sizes")
            val s = m["s"] as? String
            if (s.isNullOrEmpty()) throw CryptoException("missing server")
            val n = m["n"]?.let { it as? String ?: throw CryptoException("bad name") }
            if (n != null && n.toByteArray().size > 64) throw CryptoException("name too long")
            val i = m["i"]?.let { it as? ByteArray ?: throw CryptoException("bad invite") }
            if (i != null && i.size != 16) throw CryptoException("bad invite size")
            return Offer(e, r, s, n, i)
        }
    }
}

/**
 * Everything a contact needs, derived from root (§3.3).
 * "send" fields are for messages this device sends; "recv" for the peer's.
 */
class Keys(
    val root: ByteArray,
    val isLo: Boolean,
    val sendCk: ByteArray,
    val recvCk: ByteArray,
    /** The peer's mailbox, on the peer's server. */
    val sendMbox: ByteArray,
    /** Our mailbox, on our server. */
    val recvMbox: ByteArray,
    val sendToken: ByteArray,
    /** We register SHA-256 of this with our server (§3.4). */
    val recvToken: ByteArray,
    val confirm: List<String>,
)

/** One side of an in-person pairing in progress (§3.2). Discard it after [complete]. */
class Pairing private constructor(private val priv: ByteArray, r: ByteArray, server: String, name: String?, invite: ByteArray?) {
    val offer = Offer(X25519PrivateKeyParameters(priv, 0).generatePublicKey().encoded, r, server, name, invite)

    /** Derives the shared keys from the peer's offer. */
    fun complete(peer: Offer): Keys {
        val cmp = compareBytes(offer.e, peer.e)
        if (cmp == 0) throw CryptoException("scanned own QR code")
        val dh = ByteArray(32)
        try {
            X25519Agreement().apply { init(X25519PrivateKeyParameters(priv, 0)) }
                .calculateAgreement(X25519PublicKeyParameters(peer.e, 0), dh, 0)
        } catch (e: IllegalStateException) {
            throw CryptoException("bad peer key") // low-order point, all-zero result
        }
        if (dh.all { it == 0.toByte() }) throw CryptoException("bad peer key")
        return if (cmp < 0) deriveKeys(dh, offer, peer, true) else deriveKeys(dh, peer, offer, false)
    }

    /** Overwrites the ephemeral private key (§3.2 step 4). */
    fun destroy() = priv.fill(0)

    companion object {
        fun create(server: String, name: String? = null, invite: ByteArray? = null, rng: SecureRandom = SecureRandom()): Pairing {
            val e = ByteArray(32).also(rng::nextBytes)
            val r = ByteArray(32).also(rng::nextBytes)
            return Pairing(e, r, server, name, invite)
        }

        /** Deterministic; for test vectors. */
        fun fromSecrets(e: ByteArray, r: ByteArray, server: String, name: String? = null, invite: ByteArray? = null) =
            Pairing(e.copyOf(), r, server, name, invite)

        /** §3.3 */
        fun deriveKeys(dh: ByteArray, lo: Offer, hi: Offer, weAreLo: Boolean): Keys {
            val root = hkdf(concat(dh, lo.r, hi.r), (LABEL_PREFIX + "pair").toByteArray(), concat(lo.e, hi.e), 32)
            val (send, recv) = if (weAreLo) "lo2hi" to "hi2lo" else "hi2lo" to "lo2hi"
            return Keys(
                root = root,
                isLo = weAreLo,
                sendCk = kdf(root, "chain/$send", 32),
                recvCk = kdf(root, "chain/$recv", 32),
                sendMbox = kdf(root, "mbox/$send", 16),
                recvMbox = kdf(root, "mbox/$recv", 16),
                sendToken = kdf(root, "send/$send", 32),
                recvToken = kdf(root, "send/$recv", 32),
                confirm = Words.fromBits(kdf(root, "confirm", 5), Const.CONFIRM_WORDS),
            )
        }
    }
}
