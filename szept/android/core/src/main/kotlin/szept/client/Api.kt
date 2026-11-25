package szept.client

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONArray
import org.json.JSONObject
import java.util.Base64
import java.util.concurrent.TimeUnit

class ApiException(val code: Int, message: String) : Exception(message)

/** A message as the server delivers it (§8.5). */
class WireMsg(val mailbox: ByteArray, val id: ByteArray, val body: ByteArray)

/** The server API (§8). Server URLs are base URLs without a trailing slash. */
class Api(
    private val http: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .pingInterval(25, TimeUnit.SECONDS)
        .build(),
) {
    private val json = "application/json".toMediaType()
    private val octets = "application/octet-stream".toMediaType()

    private suspend fun call(req: Request, ok: Set<Int> = setOf(200, 201, 202, 204)): String = withContext(Dispatchers.IO) {
        http.newCall(req).execute().use { r ->
            val body = r.body?.string().orEmpty()
            if (r.code !in ok) {
                val msg = runCatching { JSONObject(body).getString("error") }.getOrDefault("HTTP ${r.code}")
                throw ApiException(r.code, msg)
            }
            body
        }
    }

    private fun bearer(d: Device) = "Bearer " + b64(d.token)

    suspend fun register(server: String, invite: ByteArray, tokenHash: ByteArray, killHash: ByteArray) {
        val body = JSONObject()
            .put("invite", b64(invite)).put("token_hash", b64(tokenHash)).put("kill_hash", b64(killHash))
        call(Request.Builder().url("$server/v1/devices").post(body.toString().toRequestBody(json)).build())
    }

    suspend fun deleteDevice(d: Device) {
        call(Request.Builder().url("${d.server}/v1/devices/me").delete().header("Authorization", bearer(d)).build())
    }

    suspend fun createInvite(d: Device): ByteArray {
        val r = call(Request.Builder().url("${d.server}/v1/invites").post(ByteArray(0).toRequestBody(json)).header("Authorization", bearer(d)).build())
        return unb64(JSONObject(r).getString("invite"))
    }

    suspend fun putMailbox(d: Device, id: ByteArray, sendHash: ByteArray) {
        val body = JSONObject().put("send_hash", b64(sendHash)).toString().toRequestBody(json)
        call(Request.Builder().url("${d.server}/v1/mailboxes/${id.hex()}").put(body).header("Authorization", bearer(d)).build())
    }

    suspend fun deleteMailbox(d: Device, id: ByteArray) {
        call(Request.Builder().url("${d.server}/v1/mailboxes/${id.hex()}").delete().header("Authorization", bearer(d)).build(), setOf(204, 404))
    }

    /** Posts an envelope to a peer's mailbox; no account needed on that server (§8.4). */
    suspend fun send(server: String, mbox: ByteArray, token: ByteArray, envelope: ByteArray) {
        call(
            Request.Builder().url("$server/v1/mailboxes/${mbox.hex()}/messages")
                .post(envelope.toRequestBody(octets)).header("Authorization", "Send " + b64(token)).build(),
        )
    }

    suspend fun inbox(d: Device, limit: Int = 100): List<WireMsg> {
        val r = call(Request.Builder().url("${d.server}/v1/inbox?limit=$limit").header("Authorization", bearer(d)).build())
        val a = JSONObject(r).getJSONArray("msgs")
        return List(a.length()) { wire(a.getJSONObject(it)) }
    }

    suspend fun ack(d: Device, ids: List<ByteArray>) {
        val body = JSONObject().put("ids", JSONArray(ids.map(::b64))).toString().toRequestBody(json)
        call(Request.Builder().url("${d.server}/v1/inbox/ack").post(body).header("Authorization", bearer(d)).build())
    }

    interface Socket {
        fun ack(id: ByteArray)
        fun close()
    }

    /** Opens the delivery WebSocket. Callbacks run on OkHttp's thread. */
    fun connect(d: Device, onOpen: () -> Unit, onMsg: (WireMsg) -> Unit, onClosed: (Throwable?) -> Unit): Socket {
        val ws = http.newWebSocket(
            Request.Builder().url("${d.server}/v1/ws").header("Authorization", bearer(d)).build(),
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: okhttp3.Response) = onOpen()

                override fun onMessage(webSocket: WebSocket, text: String) {
                    val o = runCatching { JSONObject(text) }.getOrNull() ?: return
                    if (o.optString("type") == "msg") onMsg(wire(o))
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = onClosed(null)
                override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                    webSocket.close(1000, null)
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: okhttp3.Response?) = onClosed(t)
            },
        )
        return object : Socket {
            override fun ack(id: ByteArray) {
                ws.send(JSONObject().put("type", "ack").put("ids", JSONArray(listOf(b64(id)))).toString())
            }

            override fun close() {
                ws.close(1000, null)
            }
        }
    }

    private fun wire(o: JSONObject) = WireMsg(unb64(o.getString("mailbox")), unb64(o.getString("id")), unb64(o.getString("body")))

    companion object {
        fun b64(b: ByteArray): String = Base64.getUrlEncoder().withoutPadding().encodeToString(b)
        fun unb64(s: String): ByteArray = Base64.getUrlDecoder().decode(s)
    }
}
