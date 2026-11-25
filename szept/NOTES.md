# szept: research and design notes

This file collects the research and the reasoning behind the design. The spec itself is in `PROTOCOL.md`.

## Original requirements

- Android app; the server runs on the homelab.
- For friends and family, so scaling doesn't matter.
- One-to-one and group chats.
- End-to-end encryption.
- Identity based on keys, age-style or words derived from the key. No phone numbers or email.
- Text, photos and other media.
- Nothing stored on the server.

## Why messengers are big and bulky

Sending a message from A to B was solved by IRC in 1988. The bulk comes from features people now expect:

| Feature | What it costs |
|---|---|
| History on the server | Database, search, backups, retention rules, compliance |
| Multiple devices | Keys per device and syncing between them; the source of most "unable to decrypt" bugs |
| E2EE in groups | Key rotation on join and leave, Megolm/Sender Keys/MLS, recovering keys on a new device |
| Federation | Matrix copies the full room history to every participating server and needs a state-resolution algorithm |
| Push notifications | Google FCM / Apple APNs, or a workaround for them |
| Calls | WebRTC, STUN/TURN servers, an SFU for group calls |
| Contact discovery | Phone numbers and private contact matching |
| Spam and abuse | Moderation, reporting, rate limits, phone verification |
| Enterprise features | Single sign-on, admin controls, audit logs, compliance export |

szept leaves out almost all of these.

### The EU Commission and Matrix (Oct 2026)

The European Commission rolled out Element Pro, a Matrix client, as a "sovereign backup" to Microsoft Teams for more than 40,000 staff. The desktop client reached laptops by September 2026. Staff complaints on the internal board were reported by Politico; the Commission called them "natural friction". One unconfirmed report says online/offline indicators had to be switched off because they overloaded the servers, which fits Matrix's replicated-state design.

## Messenger survey (Nov 2026)

| App | Identity | Architecture | E2EE | Server stores messages? | Notes |
|---|---|---|---|---|---|
| WhatsApp | Phone number | Central (Meta) | Signal protocol | Until delivered; backups in the cloud | ~3B users; metadata goes to Meta |
| Signal | Phone number, optional username | Central | Signal protocol (X3DH/PQXDH, Double Ratchet) | Until delivered | Reference design; impractical to self-host |
| Telegram | Phone number | Central, cloud-first | Not by default; only 1:1 "secret chats" | Yes | A cloud platform with channels and bots |
| iMessage | Apple ID / phone | Central | Yes (PQ3) | Until delivered, plus iCloud | Apple devices only |
| FB Messenger | Facebook account | Central | Signal-based, on by default since 2023 | Encrypted backups | |
| Viber | Phone number | Central | Yes | Until delivered | |
| LINE / WeChat | Phone number | Central | LINE: Letter Sealing; WeChat: none | Yes | Super-apps |
| Threema (paid) | Random 8-character ID | Central (Swiss); OnPrem for businesses | NaCl, plus Ibex for forward secrecy | Deleted after delivery | Paid option with no phone number |
| Wire | Email or username | Central, self-hostable but heavy | MLS | Encrypted | Enterprise focus |
| Element / Matrix | @user:server | Federated | Olm/Megolm, moving to MLS | Yes, full replicated history | The EU choice; heavy |
| XMPP (Conversations) | user@server | Federated; easy to self-host (Prosody) | OMEMO | Configurable | Lean, 1999-era design |
| Delta Chat | Address on a chatmail relay | Email (SMTP/IMAP) as transport | Autocrypt/OpenPGP, post-quantum since 2026 | Relays delete quickly | Dropped unencrypted email in Nov 2026; self-hostable relays |
| SimpleX | No identifiers; one queue per contact pair | SMP/XFTP relays, self-hostable | Double Ratchet plus an extra layer, post-quantum | Until received | Closest existing match to szept |
| Session | Key-derived Session ID | Onion-routed Oxen nodes | Own protocol; forward secrecy dropped, V2 restores it | Temporary swarm storage | April 2026 paper found flaws in its group protocol and blockchain |
| Briar | Keys exchanged by QR | P2P over Tor, Bluetooth or Wi-Fi | Yes | No server | Both phones must be online at the same time |
| Jami | Key-derived ID | P2P with a DHT | Yes | No server | Unreliable on mobile |
| Cwtch | Keys | P2P over Tor with untrusted relays | Yes | Relays hold ciphertext | Metadata-resistant groups |
| Olvid | No phone; keys exchanged in person | Central | Yes | Until delivered | Certified by ANSSI |
| Tox | Public-key ID | P2P | NaCl | No server | Mostly stalled |
| Wickr / Keybase | Username | Central | Yes | | Cautionary tales: consumer Wickr shut down in 2023, Keybase stalled |

**Where szept fits:** in the same family as Threema, SimpleX and Briar. A self-hosted SimpleX covers about 90% of the requirements. szept's reasons to exist are a codebase small enough to understand, and in-person pairing that is simpler than anyone else's.

## How the design evolved

1. **Plain age has no forward secrecy.** age encrypts each message to the recipient's *long-term* key, so stealing that key decrypts every recorded message. age also doesn't authenticate the sender.
2. **Meeting in person replaces most of Signal.** A mutual QR scan sets up a shared root secret, which makes X3DH, prekey servers, key directories and safety-number checks unnecessary. The root mixes `X25519 || r_A || r_B`, so someone filming the QR codes, or a future quantum computer, gets nothing on its own.
3. **Only current messages matter.** A lost phone means lost contacts, with no backup and no recovery. This removed the seed words for recovery, history sync and identity continuity.
4. **A device is a contact.** Several devices are just several contacts. Grouping them under one person can come later, purely in the UI.
5. **There is no long-term identity key at all.** Each pairing uses fresh keys, so contacts can't link you to each other by a shared key.
6. **Words are still used, but differently:** a 6-word kill code (written down once, it lets you remotely wipe a stolen phone's mailboxes) and 3 confirmation words shown after pairing.
7. **One-to-one messages carry no signatures.** Only the two phones know the shared root, so decryption proves the sender. Groups use Ed25519 signatures, because members share keys.
8. **Messaging across servers comes free.** Each receiver's mailbox lives on its own server, and senders post with a token, so no account is needed on the other server.
9. **Language choice:** Go for the server, Kotlin for Android.

## Hard parts that remain

- **Android push without Google:** UnifiedPush with a self-hosted ntfy server. The server only sends empty wake-ups.
- **No healing after a key compromise in v1:** a DH ratchet in v2 would fix it.
- **Metadata:** the server sees IP addresses and timing. Use Tor or a VPN if that matters.

## Status (2026-11-19)

- [x] `PROTOCOL.md` v1 draft
- [x] `proto/`: Go reference crypto plus test vectors (`proto/testdata/vectors.json`)
- [x] `server/` + `cmd/szeptd`: the full §8 API, SQLite, WebSocket, push, blobs, kill code, expiry; end-to-end tests pass
- [x] Android: Kotlin crypto layer (`android/crypto`, plain JVM module) matching `vectors.json` byte for byte, including the new file STREAM vectors; Bouncy Castle for primitives, hand-written HChaCha20 and a minimal deterministic CBOR codec
- [x] Android: `core` module (plain JVM): state, server API client, engine (setup, pairing, outbox, receiving, receipts, retention, contact lifecycle); integration test pairs two engines against a real `szeptd`
- [x] Android: `app` module, first version: setup by invite or by a friend's code, kill code, QR pairing with confirmation words, 1:1 text chats, read receipts, retention, delete chat, invites, wipe. State in a Keystore-encrypted file. Verified on the emulator against a scripted peer (`:core:peerBot`); **not yet run on a real phone**
- [ ] Android: push (UnifiedPush/ntfy); messages arrive only while the app is open
- [ ] Android: files and photos (§7), groups (§5), notifications
- SDK: `brew install --cask android-commandlinetools`, then `platforms;android-35` and `build-tools;35.0.0`; `android/local.properties` points at it

## Sources

- The Register: https://www.theregister.com/software/2026/10/29/matrix-takes-sovereign-messaging-into-the-realm-of-the-tech-giants/5305915
- Gizmodo: https://gizmodo.com/european-commission-staff-really-dislike-their-sovereign-alternative-to-microsoft-teams-report-2000821135
- Cybernews: https://cybernews.com/tech/european-commission-teams/
- AV Interactive: https://www.avinteractive.com/territories-news/europe/european-unions-teams-alternative-called-absolute-st-06-10-2026/
- InfoSec Today: https://www.infosectoday.io/matrix-gains-ground-in-europes-public-sector-but-replacing-teams-is-another-matter/
- SimpleX protocol: https://www.github.com/simplex-chat/simplex-chat/blob/stable/docs/SIMPLEX.md
- Help Net Security (SimpleX): https://www.helpnetsecurity.com/2026/04/29/product-showcase-simplex-chat-secure-messaging/
- Session Protocol V2: https://getsession.org/session-protocol-v2
- Privacy Guides (Session): https://www.privacyguides.org/news/2025/12/03/session-messenger-adds-pfs-pqe-and-other-improvements/
- ePrint 2026/773 (Session vulnerabilities): https://eprint.iacr.org/2026/773
- FOSDEM 2026 (Delta Chat): https://fosdem.org/2026/schedule/event/deltachat-chatmail-relays-multi-transport
- AlternativeTo (Delta Chat): https://alternativeto.net/news/2026/11/delta-chat-drops-unencrypted-email-adds-decentralized-onboarding-across-community-relays/
