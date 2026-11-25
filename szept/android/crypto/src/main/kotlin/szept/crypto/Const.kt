package szept.crypto

/** Constants from PROTOCOL.md §12; mirrors proto/const.go. */
object Const {
    const val VERSION = 1

    const val MAX_ENVELOPE = 65536
    const val MAX_PLAINTEXT = MAX_ENVELOPE - 5 - 16 // 0x01 | n:u32 | ... | tag
    const val MAX_GROUP_PLAIN = MAX_ENVELOPE - 13 - 16 - 64 // 0x02 | hint | n | sig ... | tag
    const val MAX_SKIP = 1000
    const val FILE_CHUNK = 64 shl 10
    const val MAX_BLOB = 100 shl 20
    const val KILL_CODE_WORDS = 6
    const val CONFIRM_WORDS = 3

    const val TYPE_PAIRWISE: Byte = 0x01
    const val TYPE_GROUP: Byte = 0x02

    /** Plaintext sizes after padding (§4.3). */
    val PAD_BUCKETS = intArrayOf(256, 1024, 4096, 16384, MAX_PLAINTEXT)
    val GROUP_PAD_BUCKETS = intArrayOf(256, 1024, 4096, 16384, MAX_GROUP_PLAIN)
}

open class CryptoException(message: String) : Exception(message)
class ReplayException : CryptoException("message key already used")
class TooFarException : CryptoException("too many skipped messages")
class DecryptException : CryptoException("decryption failed")
