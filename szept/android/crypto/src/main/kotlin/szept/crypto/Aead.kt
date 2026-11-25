package szept.crypto

import org.bouncycastle.crypto.InvalidCipherTextException
import org.bouncycastle.crypto.modes.ChaCha20Poly1305
import org.bouncycastle.crypto.params.AEADParameters
import org.bouncycastle.crypto.params.KeyParameter

/** ChaCha20-Poly1305 (12-byte nonce) and XChaCha20-Poly1305 (24-byte nonce). */
internal object Aead {
    const val TAG = 16

    fun seal(key: ByteArray, nonce: ByteArray, plaintext: ByteArray, aad: ByteArray): ByteArray =
        run(true, key, nonce, plaintext, aad)

    fun open(key: ByteArray, nonce: ByteArray, ciphertext: ByteArray, aad: ByteArray): ByteArray =
        try {
            run(false, key, nonce, ciphertext, aad)
        } catch (e: InvalidCipherTextException) {
            throw DecryptException()
        }

    fun xSeal(key: ByteArray, nonce: ByteArray, plaintext: ByteArray, aad: ByteArray): ByteArray {
        require(nonce.size == 24)
        return seal(hchacha20(key, nonce.copyOfRange(0, 16)), xNonce(nonce), plaintext, aad)
    }

    fun xOpen(key: ByteArray, nonce: ByteArray, ciphertext: ByteArray, aad: ByteArray): ByteArray {
        require(nonce.size == 24)
        return open(hchacha20(key, nonce.copyOfRange(0, 16)), xNonce(nonce), ciphertext, aad)
    }

    private fun xNonce(nonce: ByteArray) = ByteArray(4) + nonce.copyOfRange(16, 24)

    private fun run(enc: Boolean, key: ByteArray, nonce: ByteArray, input: ByteArray, aad: ByteArray): ByteArray {
        val c = ChaCha20Poly1305()
        c.init(enc, AEADParameters(KeyParameter(key), TAG * 8, nonce, aad))
        val out = ByteArray(c.getOutputSize(input.size))
        var n = c.processBytes(input, 0, input.size, out, 0)
        n += c.doFinal(out, n)
        return if (n == out.size) out else out.copyOf(n)
    }

    /** HChaCha20 (draft-irtf-cfrg-xchacha §2.2): derives the XChaCha subkey. */
    fun hchacha20(key: ByteArray, nonce16: ByteArray): ByteArray {
        require(key.size == 32 && nonce16.size == 16)
        val s = IntArray(16)
        s[0] = 0x61707865; s[1] = 0x3320646e; s[2] = 0x79622d32; s[3] = 0x6b206574
        for (i in 0 until 8) s[4 + i] = le32(key, 4 * i)
        for (i in 0 until 4) s[12 + i] = le32(nonce16, 4 * i)
        repeat(10) {
            qr(s, 0, 4, 8, 12); qr(s, 1, 5, 9, 13); qr(s, 2, 6, 10, 14); qr(s, 3, 7, 11, 15)
            qr(s, 0, 5, 10, 15); qr(s, 1, 6, 11, 12); qr(s, 2, 7, 8, 13); qr(s, 3, 4, 9, 14)
        }
        val out = ByteArray(32)
        for ((j, i) in intArrayOf(0, 1, 2, 3, 12, 13, 14, 15).withIndex()) {
            val w = s[i]
            out[4 * j] = w.toByte(); out[4 * j + 1] = (w ushr 8).toByte()
            out[4 * j + 2] = (w ushr 16).toByte(); out[4 * j + 3] = (w ushr 24).toByte()
        }
        return out
    }

    private fun le32(b: ByteArray, o: Int) =
        (b[o].toInt() and 0xff) or ((b[o + 1].toInt() and 0xff) shl 8) or
            ((b[o + 2].toInt() and 0xff) shl 16) or ((b[o + 3].toInt() and 0xff) shl 24)

    private fun qr(s: IntArray, a: Int, b: Int, c: Int, d: Int) {
        s[a] += s[b]; s[d] = Integer.rotateLeft(s[d] xor s[a], 16)
        s[c] += s[d]; s[b] = Integer.rotateLeft(s[b] xor s[c], 12)
        s[a] += s[b]; s[d] = Integer.rotateLeft(s[d] xor s[a], 8)
        s[c] += s[d]; s[b] = Integer.rotateLeft(s[b] xor s[c], 7)
    }
}
