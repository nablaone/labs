package szept.client

import com.google.zxing.BinaryBitmap
import com.google.zxing.MultiFormatReader
import com.google.zxing.client.j2se.BufferedImageLuminanceSource
import com.google.zxing.common.HybridBinarizer
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import java.io.File
import javax.imageio.ImageIO

/**
 * A scripted peer for testing the app on an emulator:
 *
 *   ./gradlew :core:peerBot -PbotArgs="<szept-invite:…> <file for the bot's QR text>"
 *
 * It registers, writes its pairing code to the file (paste it into the app),
 * then reads the app's QR code from `adb exec-out screencap`, confirms, and
 * answers every text with "echo: …" until stopped.
 */
fun main(args: Array<String>) = runBlocking {
    val (invite, qrFile) = args
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    // The app's server URL uses the emulator's name for the Mac (10.0.2.2); map it back.
    val http = okhttp3.OkHttpClient.Builder().addInterceptor { chain ->
        val r = chain.request()
        val u = if (r.url.host == "10.0.2.2") r.url.newBuilder().host("127.0.0.1").build() else r.url
        chain.proceed(r.newBuilder().url(u).build())
    }.build()
    val bot = Engine(MemoryStore(), scope, Api(http))
    bot.registerFrom(invite, "Echo bot")
    val session = bot.startPairing(includeInvite = false)
    File(qrFile).writeText(session.qr)
    println("bot code written to $qrFile; waiting for the app's QR on screen…")

    var peer: ScannedPeer? = null
    while (peer == null) {
        delay(2000)
        val text = runCatching { decodeScreen() }.getOrNull() ?: continue
        peer = runCatching { bot.scan(session, text) }.getOrNull()
    }
    println("words: ${peer.words.joinToString(" ")}")
    val id = bot.confirm(peer)
    bot.start()
    println("confirmed; echoing")
    val answered = mutableSetOf<String>()
    while (true) {
        delay(500)
        val c = bot.state.value.contact(id) ?: break
        for (m in c.messages.filter { !it.out && it.id !in answered }) {
            answered += m.id
            println("got: ${m.body}")
            bot.sendText(id, "echo: ${m.body}")
            bot.markRead(id)
        }
    }
    println("contact gone; bye")
    bot.stop()
}

private fun decodeScreen(): String {
    val p = ProcessBuilder("adb", "exec-out", "screencap", "-p").start()
    val img = ImageIO.read(p.inputStream) ?: error("no screenshot")
    p.waitFor()
    return MultiFormatReader().decode(BinaryBitmap(HybridBinarizer(BufferedImageLuminanceSource(img)))).text
}
