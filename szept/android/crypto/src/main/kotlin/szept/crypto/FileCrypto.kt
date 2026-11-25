package szept.crypto

import java.io.InputStream
import java.io.OutputStream
import java.security.MessageDigest
import java.security.SecureRandom

/**
 * File encryption (§7): ChaCha20-Poly1305 STREAM, the same construction as the
 * age payload. fkey is random per file, so the nonce is just
 * 11-byte big-endian chunk counter || last-chunk flag.
 */
object FileCrypto {
    fun newKey(rng: SecureRandom = SecureRandom()): ByteArray = ByteArray(32).also(rng::nextBytes)

    internal fun nonce(i: Long, last: Boolean): ByteArray {
        val n = ByteArray(12)
        var c = i
        for (j in 10 downTo 3) { n[j] = c.toByte(); c = c ushr 8 }
        if (last) n[11] = 1
        return n
    }

    /** Streams [input] to [output] encrypted; returns SHA-256 of the ciphertext. An empty file is one empty final chunk. */
    fun encrypt(fkey: ByteArray, input: InputStream, output: OutputStream): ByteArray {
        val h = MessageDigest.getInstance("SHA-256")
        chunks(input, Const.FILE_CHUNK) { i, chunk, last ->
            val ct = Aead.seal(fkey, nonce(i, last), chunk, ByteArray(0))
            h.update(ct)
            output.write(ct)
        }
        return h.digest()
    }

    /**
     * Reverses [encrypt]. Plaintext is written as each chunk authenticates, so
     * check the ciphertext's SHA-256 against the `file` message first (§7 step 4).
     */
    fun decrypt(fkey: ByteArray, input: InputStream, output: OutputStream) {
        chunks(input, Const.FILE_CHUNK + Aead.TAG) { i, chunk, last ->
            if (i == 0L && chunk.isEmpty()) throw CryptoException("truncated file")
            output.write(Aead.open(fkey, nonce(i, last), chunk, ByteArray(0)))
        }
    }

    /** Calls [f] for each chunk; a full chunk is last only if nothing follows it. */
    private inline fun chunks(input: InputStream, size: Int, f: (Long, ByteArray, Boolean) -> Unit) {
        var buf = ByteArray(size)
        var next = ByteArray(size)
        var n = readFull(input, buf)
        var i = 0L
        while (true) {
            var last = n < size
            var m = 0
            if (!last) {
                m = readFull(input, next)
                last = m == 0
            }
            f(i, buf.copyOf(n), last)
            if (last) return
            val t = buf; buf = next; next = t
            n = m
            i++
        }
    }

    private fun readFull(input: InputStream, b: ByteArray): Int {
        var n = 0
        while (n < b.size) {
            val r = input.read(b, n, b.size - n)
            if (r < 0) break
            n += r
        }
        return n
    }
}
