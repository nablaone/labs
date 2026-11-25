package szept.crypto

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class CryptoTest {
    private fun pair(): Pair<Keys, Keys> {
        val a = Pairing.create("https://a.example", "A")
        val b = Pairing.create("https://b.example")
        // Through the QR encoding, as on a phone.
        val ka = a.complete(Offer.decode(b.offer.encode()))
        val kb = b.complete(Offer.decode(a.offer.encode()))
        a.destroy(); b.destroy()
        return ka to kb
    }

    private fun msg(body: String) = Cbor.encode(mapOf("t" to "text", "body" to body))

    @Test
    fun hchacha20() {
        // draft-irtf-cfrg-xchacha-03 §2.2.1
        val key = ByteArray(32) { it.toByte() }
        val nonce = unhex("000000090000004a0000000031415927")
        assertContentEquals(
            unhex("82413b4227b27bfed30e42508a877d73a0f9e4d58a74a853c12ec41326d3ecdc"),
            Aead.hchacha20(key, nonce),
        )
    }

    @Test
    fun outOfOrderAndReplay() {
        val (ka, kb) = pair()
        val send = SendChain(ka.sendCk)
        val envs = (0 until 5).map { Envelope.sealPairwise(send, ka.sendMbox, msg("m$it")) }
        val recv = RecvChain(kb.recvCk)
        for (i in listOf(3, 0, 4, 1, 2)) {
            assertEquals("m$i", Cbor.decodeMap(Envelope.openPairwise(recv, kb.recvMbox, envs[i]))["body"])
        }
        assertEquals(0, recv.skipped.size)
        assertFailsWith<ReplayException> { Envelope.openPairwise(recv, kb.recvMbox, envs[2]) }
    }

    @Test
    fun forgedCounterDoesNotDesync() {
        val (ka, kb) = pair()
        val send = SendChain(ka.sendCk)
        val env = Envelope.sealPairwise(send, ka.sendMbox, msg("hi"))
        val forged = env.copyOf().also { it[4] = 50 } // n = 50
        val recv = RecvChain(kb.recvCk)
        assertFailsWith<DecryptException> { Envelope.openPairwise(recv, kb.recvMbox, forged) }
        assertEquals(0L, recv.n)
        assertEquals(0, recv.skipped.size)
        Envelope.openPairwise(recv, kb.recvMbox, env)
    }

    @Test
    fun tooFar() {
        val (ka, kb) = pair()
        val send = SendChain(ka.sendCk, n = Const.MAX_SKIP + 1L)
        val env = Envelope.sealPairwise(send, ka.sendMbox, msg("x"))
        assertFailsWith<TooFarException> { Envelope.openPairwise(RecvChain(kb.recvCk), kb.recvMbox, env) }
    }

    @Test
    fun wrongMailboxOrDirection() {
        val (ka, kb) = pair()
        val env = Envelope.sealPairwise(SendChain(ka.sendCk), ka.sendMbox, msg("x"))
        assertFailsWith<DecryptException> { Envelope.openPairwise(RecvChain(kb.recvCk), kb.sendMbox, env) }
        assertFailsWith<DecryptException> { Envelope.openPairwise(RecvChain(ka.recvCk), kb.recvMbox, env) }
    }

    @Test
    fun paddingBuckets() {
        for ((len, bucket) in listOf(0 to 256, 255 to 256, 256 to 1024, Const.MAX_PLAINTEXT - 1 to Const.MAX_PLAINTEXT)) {
            val p = ByteArray(len) { 7 }
            val padded = Envelope.pad(p, Const.PAD_BUCKETS)
            assertEquals(bucket, padded.size)
            assertContentEquals(p, Envelope.unpad(padded))
        }
        assertFailsWith<CryptoException> { Envelope.pad(ByteArray(Const.MAX_PLAINTEXT), Const.PAD_BUCKETS) }
        val (ka, _) = pair()
        val big = Envelope.sealPairwise(SendChain(ka.sendCk), ka.sendMbox, ByteArray(Const.MAX_PLAINTEXT - 1))
        assertEquals(Const.MAX_ENVELOPE, big.size)
    }

    @Test
    fun groupRejectsOtherSigner() {
        val gk = ByteArray(32) { 1 }
        val gid = ByteArray(16) { 2 }
        val ck = ByteArray(32) { 3 }
        val seed = ByteArray(32) { 4 }
        val other = Envelope.groupPublicKey(ByteArray(32) { 5 })
        val env = Envelope.sealGroup(SendChain(ck), gk, gid, 1, seed, msg("hej"))
        assertEquals(Const.TYPE_GROUP, env[0])
        assertFailsWith<CryptoException> { Envelope.openGroup(RecvChain(ck), gid, 1, other, env) }
        assertFailsWith<CryptoException> { Envelope.openGroup(RecvChain(ck), gid, 2, Envelope.groupPublicKey(seed), env) }
        val recv = RecvChain(ck)
        Envelope.openGroup(recv, gid, 1, Envelope.groupPublicKey(seed), env)
        assertEquals(1L, recv.n)
    }

    @Test
    fun fileTamperAndTruncation() {
        val key = FileCrypto.newKey()
        val plain = ByteArray(Const.FILE_CHUNK * 2 + 10) { (it * 7).toByte() }
        val ct = ByteArrayOutputStream().also { FileCrypto.encrypt(key, ByteArrayInputStream(plain), it) }.toByteArray()
        fun dec(b: ByteArray) = FileCrypto.decrypt(key, ByteArrayInputStream(b), ByteArrayOutputStream())
        // Dropping the final chunk leaves a non-final chunk at the end.
        assertFailsWith<DecryptException> { dec(ct.copyOf(2 * (Const.FILE_CHUNK + 16))) }
        assertFailsWith<CryptoException> { dec(ByteArray(0)) }
        assertFailsWith<DecryptException> { dec(ct.copyOf().also { it[100] = (it[100] + 1).toByte() }) }
    }

    @Test
    fun ownQrRejected() {
        val a = Pairing.create("https://a.example")
        assertFailsWith<CryptoException> { a.complete(a.offer) }
    }

    @Test
    fun offerValidation() {
        val a = Pairing.create("https://a.example")
        val o = Offer.decode(a.offer.encode())
        assertNull(o.name)
        assertNull(o.invite)
        assertFailsWith<CryptoException> { Offer.decode("szept2:AAAA") }
        assertFailsWith<CryptoException> { Offer.decode("szept1:" + "!!") }
        assertFailsWith<CryptoException> { Offer(a.offer.e, a.offer.r, "s", "x".repeat(65)).encode().let(Offer::decode) }
    }

    @Test
    fun cborRoundTrip() {
        val m = mapOf(
            "t" to "x", "neg" to -5L, "big" to (1L shl 40), "b" to true, "nil" to null,
            "list" to listOf(1L, "two", byteArrayOf(3)), "m" to mapOf("k" to 1L),
        )
        val back = Cbor.decodeMap(Cbor.encode(m))
        assertEquals(-5L, back["neg"])
        assertEquals(1L shl 40, back["big"])
        assertEquals(true, back["b"])
        assert(back.containsKey("nil") && back["nil"] == null)
        assertContentEquals(byteArrayOf(3), (back["list"] as List<*>)[2] as ByteArray)
        assertEquals(mapOf("k" to 1L), back["m"])
        // Deterministic: insertion order does not matter.
        assertContentEquals(Cbor.encode(mapOf("a" to 1, "bb" to 2)), Cbor.encode(mapOf("bb" to 2, "a" to 1)))
        assertFailsWith<Cbor.CborException> { Cbor.decode(byteArrayOf(0x5f)) } // indefinite
        assertFailsWith<Cbor.CborException> { Cbor.decode(byteArrayOf(0x45, 1)) } // truncated
        assertFailsWith<Cbor.CborException> { Cbor.decode(byteArrayOf(0x9b.toByte(), 0x7f, -1, -1, -1, -1, -1, -1, -1)) }
    }

    @Test
    fun killCodes() {
        val code = Words.newKillCode()
        assertEquals(6, code.split(" ").size)
        assertContentEquals(Words.killHash(code), Words.killHash(code.uppercase()))
        assertFailsWith<CryptoException> { Words.killHash("abandon ability able about above") }
        assertFailsWith<CryptoException> { Words.killHash("abandon ability able about above zzzz") }
    }
}
