package szept.crypto

import java.io.ByteArrayOutputStream

/**
 * The small subset of CBOR the protocol uses (§1): unsigned and negative
 * integers, byte strings, text strings, arrays, maps with text keys, booleans
 * and null. Encoding is deterministic (RFC 8949 §4.2.1): shortest-form
 * integers and lengths, map keys sorted by their encoded bytes.
 *
 * Kotlin types: Long/Int, ByteArray, String, List<*>, Map<String, *>, Boolean, null.
 * Decoding yields Long, ByteArray, String, List<Any?>, Map<String, Any?>, Boolean, null.
 */
object Cbor {
    class CborException(message: String) : CryptoException(message)

    private const val MAX_DEPTH = 16

    fun encode(value: Any?): ByteArray = ByteArrayOutputStream().also { write(it, value) }.toByteArray()

    private fun write(out: ByteArrayOutputStream, v: Any?) {
        when (v) {
            null -> out.write(0xf6)
            false -> out.write(0xf4)
            true -> out.write(0xf5)
            is Int -> write(out, v.toLong())
            is Long -> if (v >= 0) head(out, 0, v) else head(out, 1, -1 - v)
            is ByteArray -> { head(out, 2, v.size.toLong()); out.write(v) }
            is String -> v.toByteArray(Charsets.UTF_8).let { head(out, 3, it.size.toLong()); out.write(it) }
            is List<*> -> { head(out, 4, v.size.toLong()); v.forEach { write(out, it) } }
            is Map<*, *> -> {
                val entries = v.entries.map { (k, value) ->
                    if (k !is String) throw CborException("map keys must be text")
                    encode(k) to value
                }.sortedWith { a, b -> compareBytes(a.first, b.first) }
                head(out, 5, entries.size.toLong())
                for ((k, value) in entries) { out.write(k); write(out, value) }
            }
            else -> throw CborException("cannot encode ${v::class.simpleName}")
        }
    }

    private fun head(out: ByteArrayOutputStream, major: Int, n: Long) {
        val m = major shl 5
        when {
            n < 24 -> out.write(m or n.toInt())
            n < 0x100 -> { out.write(m or 24); out.write(n.toInt()) }
            n < 0x10000 -> { out.write(m or 25); be(out, n, 2) }
            n < 0x1_0000_0000L -> { out.write(m or 26); be(out, n, 4) }
            else -> { out.write(m or 27); be(out, n, 8) }
        }
    }

    private fun be(out: ByteArrayOutputStream, n: Long, bytes: Int) {
        for (i in bytes - 1 downTo 0) out.write((n ushr (8 * i)).toInt() and 0xff)
    }

    fun decode(b: ByteArray): Any? {
        val r = Reader(b)
        val v = r.value(0)
        if (r.pos != b.size) throw CborException("trailing bytes")
        return v
    }

    /** Decodes a top-level map, the shape of every protocol message. */
    @Suppress("UNCHECKED_CAST")
    fun decodeMap(b: ByteArray): Map<String, Any?> =
        decode(b) as? Map<String, Any?> ?: throw CborException("not a map")

    private class Reader(val b: ByteArray) {
        var pos = 0

        fun byte(): Int {
            if (pos >= b.size) throw CborException("truncated")
            return b[pos++].toInt() and 0xff
        }

        fun arg(info: Int): Long = when {
            info < 24 -> info.toLong()
            info in 24..27 -> {
                val len = 1 shl (info - 24)
                var n = 0L
                repeat(len) { n = (n shl 8) or byte().toLong() }
                if (len == 8 && n < 0) throw CborException("integer too large")
                n
            }
            else -> throw CborException("indefinite lengths not supported")
        }

        fun bytes(len: Long): ByteArray {
            if (len > b.size - pos) throw CborException("truncated")
            return b.copyOfRange(pos, pos + len.toInt()).also { pos += len.toInt() }
        }

        fun count(len: Long): Int {
            // Every item takes at least one byte; rejects absurd lengths early.
            if (len > b.size - pos) throw CborException("truncated")
            return len.toInt()
        }

        fun value(depth: Int): Any? {
            if (depth > MAX_DEPTH) throw CborException("nested too deeply")
            val ib = byte()
            val major = ib ushr 5
            val info = ib and 0x1f
            return when (major) {
                0 -> arg(info)
                1 -> -1 - arg(info)
                2 -> bytes(arg(info))
                3 -> String(bytes(arg(info)), Charsets.UTF_8)
                4 -> List(count(arg(info))) { value(depth + 1) }
                5 -> {
                    val n = count(arg(info))
                    val m = LinkedHashMap<String, Any?>(n)
                    repeat(n) {
                        val k = value(depth + 1) as? String ?: throw CborException("map keys must be text")
                        if (m.containsKey(k)) throw CborException("duplicate map key")
                        m[k] = value(depth + 1)
                    }
                    m
                }
                7 -> when (info) {
                    20 -> false
                    21 -> true
                    22 -> null
                    else -> throw CborException("unsupported simple value")
                }
                else -> throw CborException("tags not supported")
            }
        }
    }
}
