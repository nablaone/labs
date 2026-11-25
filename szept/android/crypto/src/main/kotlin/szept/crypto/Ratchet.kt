package szept.crypto

/** A sending hash ratchet (§4.1). Persist [ck] and [n] after every [next]. */
class SendChain(var ck: ByteArray, var n: Long = 0) {
    /** Returns (message key, counter) and advances, forgetting the old chain key. */
    fun next(): Pair<ByteArray, Long> {
        if (n > 0xFFFF_FFFFL) throw CryptoException("chain exhausted")
        val mk = hmacByte(ck, 0x01)
        val used = n
        ck = hmacByte(ck, 0x02)
        n++
        return mk to used
    }
}

/**
 * A receiving hash ratchet with skipped-key storage (§4.1).
 * Persist [ck], [n] and [skipped] after every successful decryption; the app
 * deletes skipped keys when the retention period runs out.
 */
class RecvChain(var ck: ByteArray, var n: Long = 0, val skipped: MutableMap<Long, ByteArray> = HashMap()) {
    /**
     * Returns the message key for [msgN] and a commit action. Nothing changes
     * until commit is called, which happens only after successful decryption,
     * so a forged counter cannot desynchronise the chain.
     */
    internal fun key(msgN: Long): Pair<ByteArray, () -> Unit> {
        if (msgN < n) {
            val k = skipped[msgN] ?: throw ReplayException()
            return k to { skipped.remove(msgN); Unit }
        }
        if (msgN - n > Const.MAX_SKIP) throw TooFarException()
        var c = ck
        val skip = HashMap<Long, ByteArray>()
        for (i in n until msgN) {
            skip[i] = hmacByte(c, 0x01)
            c = hmacByte(c, 0x02)
        }
        val mk = hmacByte(c, 0x01)
        val next = hmacByte(c, 0x02)
        return mk to {
            skipped.putAll(skip)
            ck = next
            n = msgN + 1
        }
    }
}

/** The AEAD key and XChaCha nonce for a message key. */
internal fun messageKeyNonce(mk: ByteArray): Pair<ByteArray, ByteArray> {
    val kn = kdf(mk, "msg", 56)
    return kn.copyOfRange(0, 32) to kn.copyOfRange(32, 56)
}
