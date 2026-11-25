package szept.crypto

import java.io.ByteArrayOutputStream
import java.security.MessageDigest
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

internal const val LABEL_PREFIX = "szept/v1/"

fun hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
    // HMAC pads short keys with zeros, so an empty key equals 32 zero bytes;
    // SecretKeySpec refuses empty keys.
    val k = if (key.isEmpty()) ByteArray(32) else key
    val mac = Mac.getInstance("HmacSHA256")
    mac.init(SecretKeySpec(k, "HmacSHA256"))
    return mac.doFinal(data)
}

fun sha256(data: ByteArray): ByteArray = MessageDigest.getInstance("SHA-256").digest(data)

/** HKDF-SHA256 (RFC 5869). An empty salt means 32 zero bytes. */
fun hkdf(ikm: ByteArray, salt: ByteArray, info: ByteArray, length: Int): ByteArray {
    require(length in 0..255 * 32)
    val prk = hmacSha256(salt, ikm)
    val out = ByteArrayOutputStream(length)
    var t = ByteArray(0)
    var i = 1
    while (out.size() < length) {
        t = hmacSha256(prk, t + info + byteArrayOf(i.toByte()))
        out.write(t)
        i++
    }
    return out.toByteArray().copyOf(length)
}

/** KDF(key, label, L) = HKDF-SHA256(ikm=key, salt="", info="szept/v1/"||label, L) (§1). */
fun kdf(key: ByteArray, label: String, length: Int): ByteArray =
    hkdf(key, ByteArray(0), (LABEL_PREFIX + label).toByteArray(), length)

/** H(key, byte) from §1. */
internal fun hmacByte(key: ByteArray, b: Int): ByteArray = hmacSha256(key, byteArrayOf(b.toByte()))

internal fun u32(n: Long): ByteArray {
    require(n in 0..0xFFFF_FFFFL)
    return byteArrayOf((n ushr 24).toByte(), (n ushr 16).toByte(), (n ushr 8).toByte(), n.toByte())
}

internal fun readU32(b: ByteArray, off: Int): Long =
    ((b[off].toLong() and 0xff) shl 24) or ((b[off + 1].toLong() and 0xff) shl 16) or
        ((b[off + 2].toLong() and 0xff) shl 8) or (b[off + 3].toLong() and 0xff)

internal fun concat(vararg parts: ByteArray): ByteArray {
    val out = ByteArray(parts.sumOf { it.size })
    var o = 0
    for (p in parts) {
        p.copyInto(out, o)
        o += p.size
    }
    return out
}

/** Unsigned lexicographic comparison, like Go's bytes.Compare. */
internal fun compareBytes(a: ByteArray, b: ByteArray): Int {
    for (i in 0 until minOf(a.size, b.size)) {
        val c = (a[i].toInt() and 0xff) - (b[i].toInt() and 0xff)
        if (c != 0) return c
    }
    return a.size - b.size
}
