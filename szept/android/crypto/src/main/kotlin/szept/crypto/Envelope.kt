package szept.crypto

import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters
import org.bouncycastle.crypto.signers.Ed25519Signer

/** Envelopes (§4.2, §5.3) and padding (§4.3). */
object Envelope {
    const val SIG_SIZE = 64

    fun pad(p: ByteArray, buckets: IntArray): ByteArray {
        val b = buckets.firstOrNull { p.size + 1 <= it } ?: throw CryptoException("plaintext too large")
        return p.copyOf(b).also { it[p.size] = 0x80.toByte() }
    }

    fun unpad(p: ByteArray): ByteArray {
        var i = p.size - 1
        while (i >= 0 && p[i] == 0.toByte()) i--
        if (i < 0 || p[i] != 0x80.toByte()) throw CryptoException("bad padding")
        return p.copyOf(i)
    }

    private fun seal(mk: ByteArray, plaintext: ByteArray, aad: ByteArray): ByteArray {
        val (key, nonce) = messageKeyNonce(mk)
        return Aead.xSeal(key, nonce, plaintext, aad)
    }

    private fun open(mk: ByteArray, ct: ByteArray, aad: ByteArray): ByteArray {
        val (key, nonce) = messageKeyNonce(mk)
        return Aead.xOpen(key, nonce, ct, aad)
    }

    /** Encrypts a CBOR message for the peer. [mbox] is the receiver's mailbox ([Keys.sendMbox]). */
    fun sealPairwise(c: SendChain, mbox: ByteArray, message: ByteArray): ByteArray {
        val padded = pad(message, Const.PAD_BUCKETS)
        val (mk, n) = c.next()
        val hdr = byteArrayOf(Const.TYPE_PAIRWISE) + u32(n)
        return hdr + seal(mk, padded, concat(byteArrayOf(Const.TYPE_PAIRWISE), mbox, u32(n)))
    }

    /** Decrypts an envelope from our mailbox ([Keys.recvMbox]). */
    fun openPairwise(c: RecvChain, mbox: ByteArray, env: ByteArray): ByteArray {
        if (env.size < 5 + Aead.TAG || env[0] != Const.TYPE_PAIRWISE) throw CryptoException("not a pairwise envelope")
        val n = readU32(env, 1)
        val (mk, commit) = c.key(n)
        val padded = open(mk, env.copyOfRange(5, env.size), concat(byteArrayOf(Const.TYPE_PAIRWISE), mbox, u32(n)))
        val msg = unpad(padded)
        commit()
        return msg
    }

    /** Ed25519 public key for a 32-byte signing seed. */
    fun groupPublicKey(seed: ByteArray): ByteArray = Ed25519PrivateKeyParameters(seed, 0).generatePublicKey().encoded

    /** The 8-byte sender hint. */
    fun groupHint(gk: ByteArray, sPub: ByteArray): ByteArray = hmacSha256(gk, sPub).copyOf(8)

    private fun groupSigInput(gid: ByteArray, epoch: Long, n: Long, padded: ByteArray) =
        concat((LABEL_PREFIX + "gsig").toByteArray(), gid, u32(epoch), u32(n), padded)

    /** Encrypts one group message; the result is posted to every member's group mailbox. */
    fun sealGroup(c: SendChain, gk: ByteArray, gid: ByteArray, epoch: Long, signSeed: ByteArray, message: ByteArray): ByteArray {
        val padded = pad(message, Const.GROUP_PAD_BUCKETS)
        val (mk, n) = c.next()
        val sk = Ed25519PrivateKeyParameters(signSeed, 0)
        val hint = groupHint(gk, sk.generatePublicKey().encoded)
        val sig = Ed25519Signer().run {
            init(true, sk)
            val m = groupSigInput(gid, epoch, n, padded)
            update(m, 0, m.size)
            generateSignature()
        }
        val hdr = concat(byteArrayOf(Const.TYPE_GROUP), hint, u32(n))
        return hdr + seal(mk, sig + padded, hdr)
    }

    class GroupHeader(val hint: ByteArray, val n: Long)

    /** Parses the hint and counter so the receiver can find the sender. */
    fun groupHeader(env: ByteArray): GroupHeader {
        if (env.size < 13 + Aead.TAG + SIG_SIZE || env[0] != Const.TYPE_GROUP) throw CryptoException("not a group envelope")
        return GroupHeader(env.copyOfRange(1, 9), readU32(env, 9))
    }

    /** Decrypts and verifies a group envelope from a known sender. */
    fun openGroup(c: RecvChain, gid: ByteArray, epoch: Long, sPub: ByteArray, env: ByteArray): ByteArray {
        val n = groupHeader(env).n
        val (mk, commit) = c.key(n)
        val p = open(mk, env.copyOfRange(13, env.size), env.copyOfRange(0, 13))
        if (p.size < SIG_SIZE) throw DecryptException()
        val sig = p.copyOfRange(0, SIG_SIZE)
        val padded = p.copyOfRange(SIG_SIZE, p.size)
        val ok = Ed25519Signer().run {
            init(false, Ed25519PublicKeyParameters(sPub, 0))
            val m = groupSigInput(gid, epoch, n, padded)
            update(m, 0, m.size)
            verifySignature(sig)
        }
        if (!ok) throw CryptoException("bad signature")
        val msg = unpad(padded)
        commit()
        return msg
    }
}
