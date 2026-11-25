package szept.crypto

import java.security.SecureRandom
import java.util.Locale

/** BIP39 English wordlist: confirmation words (§3.3) and kill codes (§2). */
object Words {
    val list: List<String> = Words::class.java.getResourceAsStream("wordlist_en.txt")!!
        .bufferedReader().readText().split(Regex("\\s+")).filter { it.isNotEmpty() }
        .also { check(it.size == 2048) }

    private val index: Map<String, Int> = list.withIndex().associate { it.value to it.index }

    /** The first 11*count bits of b (big-endian) as words. */
    fun fromBits(b: ByteArray, count: Int): List<String> = List(count) { i ->
        var idx = 0
        for (j in 0 until 11) {
            val bit = i * 11 + j
            idx = (idx shl 1) or ((b[bit / 8].toInt() ushr (7 - bit % 8)) and 1)
        }
        list[idx]
    }

    /** Lowercases and collapses whitespace; every word must be in the list. */
    fun normalizeKillCode(code: String): String {
        val f = code.lowercase(Locale.ROOT).split(Regex("\\s+")).filter { it.isNotEmpty() }
        if (f.size != Const.KILL_CODE_WORDS) throw CryptoException("kill code must have 6 words")
        if (f.any { it !in index }) throw CryptoException("unknown word in kill code")
        return f.joinToString(" ")
    }

    /** SHA-256 of the normalized kill code; sent to the server as kill_hash. */
    fun killHash(code: String): ByteArray = sha256(normalizeKillCode(code).toByteArray())

    /** 6 random words (66 bits). */
    fun newKillCode(rng: SecureRandom = SecureRandom()): String =
        fromBits(ByteArray(9).also(rng::nextBytes), Const.KILL_CODE_WORDS).joinToString(" ")
}
