# szept protocol, version 1 (draft)

szept is a small messenger for friends and family. You run the server on your homelab, and phones pair by meeting in person.

## 0. Design principles

1. **A device is a contact.** There are no accounts, phone numbers, emails or global user IDs.
2. **Contacts are created in person.** Two phones scan each other's QR codes, and that is the only way to add a contact.
3. **Only current messages matter.** Messages expire on the phones, and the server deletes them once delivered. There is no history, no backup and no recovery.
4. **A lost phone means lost contacts.** You pair again with a new phone.
5. **The server is a dumb mailbox.** It stores only ciphertext, and only until the recipient confirms receipt or the time limit (TTL) runs out.
6. **Keep it small.** Each piece of the design should be easy to explain, and to delete.

Out of scope for v1: history sync, linked devices, calls, federation logic, contact discovery, usernames.

---

## 1. Primitives

| Purpose | Primitive |
|---|---|
| Key agreement | X25519 |
| Signatures (groups only) | Ed25519 |
| KDF | HKDF-SHA256 |
| Hash ratchet | HMAC-SHA256 |
| Message encryption | XChaCha20-Poly1305 |
| File encryption | ChaCha20-Poly1305 in the STREAM construction, the same as the age payload format (§7) |
| Plaintext encoding | Deterministic CBOR (RFC 8949 §4.2) |
| Words | BIP39 English wordlist (2048 words, 11 bits per word) |

Notation:

```
KDF(key, label, L) = HKDF-SHA256(ikm = key, salt = "", info = "szept/v1/" || label, L)
H(key, byte)       = HMAC-SHA256(key, [byte])
||                 = concatenation
u32                = 4-byte unsigned integer, big-endian
```

---

## 2. Device setup

On first launch the app does the following:

1. Generates a random **storage key**, wrapped by the Android Keystore (StrongBox if available). The local database is encrypted with it. Set `allowBackup="false"` so nothing goes to cloud backups.
2. Registers with its **home server** (§8.1) using an invite. It sends:
   - `device_token`: 32 random bytes, kept on the phone. The server stores only `SHA-256(device_token)`.
   - `kill_hash`: the hash of a **kill code**.
3. Shows the **kill code once**: 6 BIP39 words (66 bits). The user writes them down or skips this step. Entering the kill code on the server deletes the device and all its mailboxes (§8.7).

There is **no long-term identity key.** Every pairing uses fresh keys (§3), so two contacts cannot compare notes and link you by a shared key.

---

## 3. Pairing (in person)

### 3.1 QR payload

Each phone generates fresh pairing material for every pairing:

- `e`, `E`: a new X25519 key pair
- `r`: 32 random bytes

The QR code contains the URI `szept1:` followed by the base64url encoding (no padding) of a CBOR map:

| Key | Type | Meaning |
|---|---|---|
| `v` | uint | Protocol version, `1` |
| `E` | bytes(32) | Ephemeral X25519 public key |
| `r` | bytes(32) | Random secret |
| `s` | text | This device's home server base URL, e.g. `https://szept.example.lan` |
| `n` | text | Suggested display name (optional, at most 64 bytes) |
| `i` | bytes(16) | Invite token for `s`, given to a peer who has no server yet (optional) |

The payload is about 150 bytes plus the URL, so it fits in a small QR code.

### 3.2 Ceremony

1. Both phones open **Pair** and show their QR codes.
2. A scans B's code, and B scans A's code. The order doesn't matter.
3. Both phones derive the root key (§3.3) and show **3 confirmation words**. The two people check that the words match.
4. Both phones **delete `e`**. From now on the contact is just `root`.
5. Each phone registers its receive mailbox for this contact (§3.4) and sends a `hello` (§6). The contact stays *pending* until the peer's `hello` arrives.

If a peer has no server yet, it registers with the server from `i` first. Pairing over a video call works the same way, but security is weaker: anyone who records the call sees `r`.

### 3.3 Key derivation

Sort the two sides by their `E` (lexicographic byte order) into **lo** and **hi**.

```
dh   = X25519(e_self, E_peer)
root = HKDF-SHA256(ikm  = dh || r_lo || r_hi,
                   salt = "szept/v1/pair",
                   info = E_lo || E_hi,
                   L    = 32)
```

`root` stays secret unless an attacker has **both** the DH result **and** both `r` values:

- Filming the QR codes gives you `r` but not `dh`.
- A future quantum computer could break `dh` but would not have `r`.

For each direction `d` in {`lo2hi`, `hi2lo`}, where `lo2hi` means lo sends and hi receives:

```
ck0[d]     = KDF(root, "chain/" || d, 32)     # starting chain key for the hash ratchet
mbox[d]    = KDF(root, "mbox/"  || d, 16)     # mailbox ID on the receiver's server
send[d]    = KDF(root, "send/"  || d, 32)     # token that allows posting to that mailbox
confirm    = KDF(root, "confirm", 5)          # first 33 bits → 3 BIP39 words
```

### 3.4 Mailboxes

- The **receiver** owns the mailbox for its incoming direction. It creates it on **its own home server** with `PUT /v1/mailboxes/{mbox[d]}` and `send_hash = SHA-256(send[d])` (§8.3).
- The **sender** posts to `peer_server/v1/mailboxes/{mbox[d]}/messages` using `send[d]`.

The two peers can use **different servers**, because each side only needs the other's URL. This gives cross-server messaging without any federation protocol.

---

## 4. Pairwise messages

### 4.1 Hash ratchet

Each direction has its own chain:

```
mk[n]   = H(ck[n], 0x01)     # message key for message n
ck[n+1] = H(ck[n], 0x02)
key || nonce = KDF(mk[n], "msg", 56)   # 32-byte key, 24-byte XChaCha nonce
```

- The sender increments `n` for every message, and deletes `mk[n]` and `ck[n]` once they are used.
- The receiver may skip ahead **at most 1000** messages. It keeps the keys for skipped messages until the conversation's retention period runs out, then deletes them.
- A message with an `n` that has already been consumed is rejected, which blocks replays.

**Properties:** a stolen phone does not reveal messages that were already deleted. This is forward secrecy. Recovering after a compromise would need a DH ratchet, which v1 does not include (§11).

### 4.2 Envelope (what the server sees)

```
0x01 | n: u32 | XChaCha20-Poly1305(key, nonce, plaintext, aad)

aad = 0x01 || mbox[d] || n
```

The maximum envelope size is **65,536 bytes**.

There are no signatures. Only the two paired phones know `root`, so a valid AEAD tag already proves who sent the message. This also means the receiver cannot prove to anyone else what was sent, which is intended.

### 4.3 Padding

The plaintext is a CBOR-encoded message (§6), then `0x80`, then zero bytes up to the next bucket: 256, 1024, 4096, 16384 or 65515 bytes. The receiver strips trailing zeros and then the `0x80`.

Group messages use the same buckets, except that the last one is **65443** bytes, because the group header and the 64-byte signature need room inside the 65,536-byte envelope.

---

## 5. Groups

The person who creates a group is its **admin**, and only the admin can change membership. If the admin loses their phone, the group is gone, and someone creates a new one. That is consistent with principle 4.

### 5.1 Per-member state

Each member device has the following for each group:

| Item | Created by | Purpose |
|---|---|---|
| `S`, `S_pub` | member | Ed25519 signing key pair; `S_pub` is the member ID |
| `gmbox`, `gsend` | member | The member's receive mailbox for this group, and its send token |
| `ck` | **admin** | The member's sending chain key (a hash ratchet, as in §4.1) |
| `gk` | **admin** | Group key, used only to compute sender hints |

The admin knows every member's chain key. That's acceptable because the admin is a member and already reads everything. The admin **cannot impersonate** other members, because it doesn't have their `S`.

### 5.2 Flows

All of these messages travel over **pairwise** channels between the admin and each member. Members don't need to have met each other.

- **Create or add:**
  1. The admin sends `group_invite`.
  2. The invitee creates `gmbox` on its server and replies with `group_join`: `{gid, S_pub, gmbox, gsend, server, name}`.
  3. The admin sends a `group_roster` to every member.
- **Remove:** the admin creates a new `gk` and a new `ck` for every remaining member, increments `epoch`, and sends the roster. The removed member keeps the old keys, so it can't read new messages, and its messages fail the hint lookup.
- **Leave:** the member sends `group_leave` to the admin, and the admin removes them.

The roster is sent with `epoch`. Each member entry contains `{S_pub, name, server, gmbox, gsend, ck, n}`, and the recipient's own entry includes its `ck`.

A new member receives the current chain keys. Earlier messages were deleted from the server after delivery, so it can't fetch them.

### 5.3 Group envelope

```
hint = HMAC-SHA256(gk, S_pub)[0..8]
sig  = Ed25519(S, "szept/v1/gsig" || gid || epoch:u32 || n:u32 || padded_plaintext)

0x02 | hint(8) | n: u32 | XChaCha20-Poly1305(key, nonce, sig || padded_plaintext, aad = 0x02 || hint || n)
```

- The sender encrypts **once** and posts the same envelope to every other member's `gmbox`. For a family-sized group that's N small HTTP POSTs.
- The receiver uses `hint` to find the sender, derives the key at `n`, decrypts, and then **verifies `sig`** against that sender's `S_pub`. If any step fails, it drops the message.

---

## 6. Message payloads

Every plaintext is a CBOR map with these common fields:

| Key | Type | Meaning |
|---|---|---|
| `t` | text | Message type |
| `id` | bytes(16) | Random message ID |
| `ts` | uint | Sender's clock in Unix milliseconds (informational only) |

| `t` | Fields | Notes |
|---|---|---|
| `hello` | `name`, `ret` | First message after pairing. `ret` is the proposed retention in seconds. |
| `text` | `body`, `re?` | `re` is the ID of the message being replied to. |
| `file` | `url`, `key`, `size`, `mime`, `name?`, `sha256`, `thumb?` | Blob reference (§7). `thumb` is a JPEG of at most 8 KiB. |
| `rcpt` | `ids`, `k` | `k` is `"d"` for delivered or `"r"` for read. |
| `ret` | `sec` | Changes the retention timer. The most recent change wins and is shown in the chat. |
| `alive` | | Sent if the device has sent nothing for 72 hours (§9). |
| `bye` | | The contact was deleted. The peer deletes it too. |
| `name` | `name` | Display name changed. |
| `group_invite` | `gid`, `name` | Pairwise only. |
| `group_join` | `gid`, `S_pub`, `gmbox`, `gsend`, `server`, `name` | Pairwise only. |
| `group_roster` | `gid`, `name`, `epoch`, `gk`, `members[]` | Pairwise only, sent by the admin. |
| `group_leave` | `gid` | Pairwise only. |

In a group, messages of type `text`, `file`, `rcpt` and `ret` also carry `gid`.

Clients **ignore unknown `t` values and unknown fields**. This is how the protocol stays extensible.

**Retention:** the default is 7 days. When a message is older than `ret`, both phones delete it, along with any downloaded file and any skipped keys. Deletion relies on honest clients; screenshots and modified apps are out of scope.

---

## 7. Files and media

1. The sender generates `fkey` (32 random bytes) and encrypts the file in **64 KiB chunks** using ChaCha20-Poly1305 STREAM, exactly as in the age payload format. The 12-byte nonce is `chunk_index (11-byte big-endian) || last_flag` (`0x01` on the final chunk, otherwise `0x00`). Because `fkey` is used for only one file, no random nonce prefix is needed. An empty file is a single empty final chunk. Truncation is detected because the final chunk must carry `last_flag = 1`.
2. It uploads the blob to its **own** home server (§8.6) and gets back a URL.
3. It sends a `file` message containing the URL, `fkey` and `sha256` of the ciphertext.
4. The receiver downloads the blob, checks `sha256`, and decrypts it.

The server deletes a blob after `max_downloads` (the number of recipients) or **7 days**, whichever comes first. The maximum blob size is **100 MiB**. Senders should resize photos before sending.

---

## 8. Server API

The server speaks HTTPS with JSON, except that message and blob bodies are `application/octet-stream`. Identifiers are lowercase hex in paths and base64url in JSON.

Authorization headers:

- `Authorization: Bearer <device_token>`: device operations.
- `Authorization: Send <send_token>`: posting to a mailbox. No device account is needed, so senders from other servers can post.

### 8.1 Devices

```
POST /v1/devices
  { "invite": b64, "token_hash": b64, "kill_hash": b64, "push_url": "https://…" | null }
  → 201 | 403 (invalid invite)

PATCH  /v1/devices/me  { "push_url": … }        (Bearer)
DELETE /v1/devices/me                            (Bearer)  → deletes the device and its mailboxes
```

### 8.2 Invites

```
POST /v1/invites  (Bearer)  → 201 { "invite": b64, "expires": ts }
```

- An invite is single-use and expires after 7 days.
- A device may have at most 5 open invites.
- The admin can also create invites from the CLI (`szeptd invite`), which is how the first device is set up. The CLI prints `szept-invite:<server URL>#<invite base64url>`, which the app accepts as a QR code or as pasted text.

### 8.3 Mailboxes

```
PUT    /v1/mailboxes/{id}   (Bearer)  { "send_hash": b64 }  → 201 | 409 (exists)
DELETE /v1/mailboxes/{id}   (Bearer, owner only)
```

The server stores `{id, owner_device, send_hash}`.

### 8.4 Sending

```
POST /v1/mailboxes/{id}/messages   (Send)   body = envelope (≤ 65536 B)
  → 202 { "id": b64 }
  → 403 (wrong token) | 404 (no such mailbox) | 413 (too large) | 429 (rate limited)
```

A 404 means the contact is gone, either deleted or killed. The client shows that to the user.

### 8.5 Receiving

WebSocket: `GET /v1/ws` (Bearer)

```
server → client   { "type": "msg", "mailbox": b64, "id": b64, "ts": ms, "body": b64 }
client → server   { "type": "ack", "ids": [b64, …] }
```

After connecting, the server delivers everything queued, then new messages as they arrive. **An acknowledged message is deleted immediately.**

Polling fallback:

```
GET  /v1/inbox?limit=100   (Bearer)  → { "msgs": [ … ] }
POST /v1/inbox/ack         (Bearer)  { "ids": [ … ] }
```

**Push:** if `push_url` is set (a UnifiedPush endpoint, e.g. from your own ntfy server), the server sends an **empty** POST to it whenever an owned mailbox receives a message. The push contains no data. It only tells the app to connect.

### 8.6 Blobs

```
POST /v1/blobs   (Bearer)   headers: X-Max-Downloads: N   body = ciphertext (≤ 100 MiB)
  → 201 { "url": "https://server/v1/blobs/<128-bit id>" }

GET  /v1/blobs/{id}         (no auth; the ID is unguessable)  → 200 | 404
```

### 8.7 Kill

```
POST /v1/kill  { "code": "word1 word2 … word6" }  → always 204
```

If `SHA-256(normalized code)` matches a device, the server deletes that device and all its mailboxes and blobs. The response is always the same, and the endpoint is rate limited per IP.

### 8.8 Server rules

- **Undelivered messages expire after 7 days.**
- Message queues may live on disk, but a message is deleted when it is acknowledged or expires. A server that keeps queues only in memory is allowed; pending messages are lost on restart.
- **Don't log mailbox IDs, tokens or bodies.**
- Rate limits: sending is limited per send token and per IP; registration and kill are limited per IP.

---

## 9. Contact lifecycle

```
pending ──hello──▶ active ──no inbound for 14 days──▶ quiet ──no inbound for 60 days──▶ deleted
                     ▲                                  │
                     └───────── any message ────────────┘
```

- A device sends `alive` if it has sent nothing to that contact for 72 hours. Silence therefore means the phone is gone.
- A **quiet** contact shows a warning such as "not seen for 14 days".
- When a contact is **deleted**, the app sends `bye` (best effort), deletes its own receive mailbox, and erases all keys for that contact.
- If a send returns **404**, the contact is deleted immediately.
- **Multiple devices:** a later client version may group several contacts under one person in the UI and send to all of them. The protocol doesn't change.

---

## 10. Threat model

**Protected:**

- Message and file contents, from the server, the network, and anyone who films the QR codes.
- Impersonation by a stranger. Pairing happens in person, so man-in-the-middle attacks aren't possible.
- Messages that have been deleted, even if the phone is later seized (forward secrecy through the hash ratchet and deletion).
- Content against a future quantum attacker, **provided `r` was not observed** during pairing.
- Linking one person's contacts by key, because there is no global identifier.
- One group member impersonating another (Ed25519 signatures).

**Not protected:**

- **Metadata the server can see:** IP addresses, timing, message sizes (after padding), and which mailboxes belong to which device. It's your server. Use Tor or a VPN if this matters.
- An **unlocked phone** in an attacker's hands: they can read current messages and send as you. Mitigations are the app lock and the kill code.
- A **malicious contact** who screenshots or keeps copies of messages.
- Recovery after an attacker has silently copied chain keys. There is no post-compromise security in v1, because there is no DH ratchet.
- Denial of service against the homelab.

---

## 11. Future (v2 candidates)

- **DH ratchet:** add an ephemeral X25519 key to every message, which turns the hash ratchet into a full Double Ratchet. The in-person `root` replaces X3DH.
- **Mailbox rotation:** `mbox[d, epoch]`, so mailbox IDs change over time.
- **Batch send:** `POST /v1/send` to deliver one group envelope to many mailboxes in a single request.
- **Person grouping:** combine several devices into one contact in the UI.
- **Header encryption:** hide `n` from the server.

---

## 12. Constants

| Name | Value |
|---|---|
| Max envelope | 65,536 B |
| Padding buckets | 256, 1024, 4096, 16384, 65515 |
| Max skipped messages | 1,000 |
| Message TTL on server | 7 days |
| Blob TTL / max size | 7 days / 100 MiB |
| File chunk size | 64 KiB |
| Default retention | 7 days |
| `alive` interval | 72 h |
| Quiet / deleted | 14 / 60 days |
| Invite expiry / max open | 7 days / 5 |
| Kill code | 6 BIP39 words |
| Confirmation | 3 BIP39 words |

---

## 13. TODO before implementation

- [x] Test vectors for §3.3, §4.1 and §5.3: `proto/testdata/vectors.json`, regenerated with `go test ./proto -run TestVectors -update`.
- [x] Normalization rule for kill codes: lowercase, words separated by single spaces, exactly 6 words, every word in the BIP39 list. The hash is SHA-256 of that string.
- [ ] Push URL policy: the server accepts only `https://` URLs, optionally limited to prefixes given with `-push-allow`. This prevents server-side request forgery (SSRF) into the homelab.
- [ ] Exact CBOR field types for `group_roster.members`.
- [ ] Decide on reaction, edit and delete messages, or leave them out.
