package szept.client

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import java.io.File
import java.net.HttpURLConnection
import java.net.ServerSocket
import java.net.URI
import java.nio.file.Files
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Two engines against a real szeptd built from this repo. */
class EngineTest {
    private lateinit var proc: Process
    private lateinit var url: String
    private lateinit var tmp: File
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    @BeforeTest
    fun startServer() {
        val repo = File(System.getProperty("szeptRepo"))
        tmp = Files.createTempDirectory("szept-test").toFile()
        val bin = File(tmp, "szeptd")
        run(repo, "go", "build", "-o", bin.path, "./cmd/szeptd")
        val port = ServerSocket(0).use { it.localPort }
        url = "http://127.0.0.1:$port"
        proc = ProcessBuilder(bin.path, "serve", "-data", File(tmp, "data").path, "-addr", "127.0.0.1:$port", "-url", url)
            .redirectErrorStream(true).redirectOutput(File(tmp, "server.log")).start()
        repeat(100) {
            val ok = runCatching {
                (URI("$url/v1/health").toURL().openConnection() as HttpURLConnection).responseCode == 200
            }.getOrDefault(false)
            if (ok) return
            Thread.sleep(100)
        }
        error("server did not start")
    }

    @AfterTest
    fun stopServer() {
        scope.cancel()
        proc.destroy()
        proc.waitFor()
        tmp.deleteRecursively()
    }

    private fun run(dir: File, vararg cmd: String): String {
        val p = ProcessBuilder(*cmd).directory(dir).redirectErrorStream(true).start()
        val out = p.inputStream.bufferedReader().readText()
        check(p.waitFor() == 0) { "${cmd.joinToString(" ")}: $out" }
        return out
    }

    private fun adminInvite(): String =
        run(tmp, File(tmp, "szeptd").path, "invite", "-data", File(tmp, "data").path, "-url", url)
            .lines().first { it.startsWith(Engine.INVITE_PREFIX) }

    private suspend fun Engine.until(what: String, cond: (AppState) -> Boolean) {
        try {
            withTimeout(10_000) { state.first(cond) }
        } catch (e: Exception) {
            throw AssertionError("timed out waiting for $what: ${state.value}", e)
        }
    }

    @Test
    fun pairChatAndBye() = runBlocking {
        val storeA = MemoryStore()
        val storeB = MemoryStore()
        val a = Engine(storeA, scope)
        val b = Engine(storeB, scope)

        // A uses the admin invite; B has no server and uses the invite inside A's QR code.
        val kill = a.registerFrom(adminInvite(), "Alice")
        assertEquals(6, kill.split(" ").size)
        val sa = a.startPairing(includeInvite = true)
        b.registerFrom(sa.qr, "Bob")
        assertEquals(url, b.state.value.device?.server)
        val sb = b.startPairing(includeInvite = false)

        val pa = a.scan(sa, sb.qr)
        val pb = b.scan(sb, sa.qr)
        assertEquals(pa.words, pb.words)
        assertEquals("Bob", pa.peer.name)

        // A confirms first; its hello hits a 404 until B creates its mailbox, then retries.
        a.start(); b.start()
        val ca = a.confirm(pa)
        delay(200)
        val cb = b.confirm(pb)
        a.flush()
        a.until("A active") { it.contact(ca)?.active == true }
        b.until("B active") { it.contact(cb)?.active == true }
        assertEquals("Alice", b.state.value.contact(cb)?.name)

        a.sendText(ca, "cześć")
        b.until("B gets text") { s -> s.contact(cb)?.messages?.any { it.body == "cześć" } == true }
        assertEquals(1, b.state.value.contact(cb)?.unread)
        a.until("A sees delivered") { s -> s.contact(ca)?.messages?.single()?.status == MsgStatus.DELIVERED }

        b.markRead(cb)
        a.until("A sees read") { s -> s.contact(ca)?.messages?.single()?.status == MsgStatus.READ }

        // B restarts from its stored state and keeps the conversation.
        b.stop()
        val b2 = Engine(storeB, scope)
        assertEquals(1, b2.state.value.contact(cb)?.messages?.size)
        a.sendText(ca, "still there?")
        b2.start()
        b2.until("B2 gets text") { s -> s.contact(cb)?.messages?.any { it.body == "still there?" } == true }
        b2.sendText(cb, "yes")
        a.until("A gets reply") { s -> s.contact(ca)?.messages?.any { it.body == "yes" && !it.out } == true }

        // A deletes the chat; B gets bye and drops the contact. Its mailbox goes too.
        a.deleteContact(ca)
        assertNull(a.state.value.contact(ca))
        b2.until("B drops contact") { it.contact(cb) == null }
    }

    @Test
    fun badCodes() = runBlocking {
        val a = Engine(MemoryStore(), scope)
        assertFailsWith<IllegalArgumentException> { a.registerFrom("szept-invite:$url#AAAA", "x") }
        assertFailsWith<ApiException> { a.registerFrom("szept-invite:$url#AAAAAAAAAAAAAAAAAAAAAA", "x") }
        assertNull(a.state.value.device)
        a.registerFrom(adminInvite(), "A")
        val s = a.startPairing(false)
        assertFailsWith<Exception> { a.scan(s, s.qr) }
        val other = Engine(MemoryStore(), scope)
        // A pairing code without an invite can't be used for setup.
        assertFailsWith<IllegalArgumentException> { other.registerFrom(s.qr, "B") }
        assertNotNull(a.state.value.device)
        assertTrue(Codec.decode(Codec.encode(a.state.value)).device!!.token.contentEquals(a.state.value.device!!.token))
    }
}
