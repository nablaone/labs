package szept.client

import szept.crypto.Cbor

/** AppState <-> CBOR, reusing the protocol's codec. Field names are short and stable. */
internal object Codec {
    private const val VERSION = 1L

    fun encode(s: AppState): ByteArray = Cbor.encode(
        mapOf(
            "v" to VERSION,
            "device" to s.device?.let { mapOf("server" to it.server, "token" to it.token, "name" to it.name) },
            "contacts" to s.contacts.map(::contact),
        ),
    )

    private fun contact(c: Contact) = mapOf(
        "id" to c.id, "name" to c.name, "server" to c.peerServer,
        "smbox" to c.sendMbox, "stok" to c.sendToken, "rmbox" to c.recvMbox,
        "send" to chain(c.send), "recv" to chain(c.recv),
        "active" to c.active, "created" to c.created, "in" to c.lastIn, "out" to c.lastOut,
        "ret" to c.retention, "unread" to c.unread.toLong(),
        "msgs" to c.messages.map { mapOf("id" to it.id, "out" to it.out, "body" to it.body, "ts" to it.ts, "st" to it.status.name) },
        "outbox" to c.outbox.map { mapOf("env" to it.envelope, "mid" to it.msgId, "q" to it.queued) },
    )

    private fun chain(c: ChainState) = mapOf(
        "ck" to c.ck, "n" to c.n,
        "skip" to c.skipped.map { (n, k) -> listOf(n, k, c.skippedAt[n] ?: 0L) },
    )

    fun decode(b: ByteArray): AppState {
        val m = Cbor.decodeMap(b)
        require(m["v"] == VERSION) { "unknown state version" }
        val d = m["device"]?.let { it as Map<*, *> }
        return AppState(
            device = d?.let { Device(it.s("server"), it.b("token"), it.s("name")) },
            contacts = (m["contacts"] as List<*>).map { contact(it as Map<*, *>) },
        )
    }

    private fun contact(m: Map<*, *>) = Contact(
        id = m.s("id"), name = m.s("name"), peerServer = m.s("server"),
        sendMbox = m.b("smbox"), sendToken = m.b("stok"), recvMbox = m.b("rmbox"),
        send = chain(m["send"] as Map<*, *>), recv = chain(m["recv"] as Map<*, *>),
        active = m["active"] as Boolean, created = m.l("created"), lastIn = m.l("in"), lastOut = m.l("out"),
        retention = m.l("ret"), unread = m.l("unread").toInt(),
        messages = (m["msgs"] as List<*>).map {
            val x = it as Map<*, *>
            ChatMessage(x.s("id"), x["out"] as Boolean, x.s("body"), x.l("ts"), MsgStatus.valueOf(x.s("st")))
        },
        outbox = (m["outbox"] as List<*>).map {
            val x = it as Map<*, *>
            Outgoing(x.b("env"), x["mid"] as String?, x.l("q"))
        },
    )

    private fun chain(m: Map<*, *>): ChainState {
        val skip = (m["skip"] as List<*>).map { it as List<*> }
        return ChainState(
            ck = m.b("ck"), n = m.l("n"),
            skipped = skip.associate { (it[0] as Long) to (it[1] as ByteArray) },
            skippedAt = skip.associate { (it[0] as Long) to (it[2] as Long) },
        )
    }

    private fun Map<*, *>.s(k: String) = this[k] as String
    private fun Map<*, *>.b(k: String) = this[k] as ByteArray
    private fun Map<*, *>.l(k: String) = this[k] as Long
}
