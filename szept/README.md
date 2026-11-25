# szept

A small end-to-end encrypted messenger for friends and family. The server runs on your homelab, and contacts are added by meeting in person.

- `PROTOCOL.md`: the protocol spec, and the source of truth for the design.
- `NOTES.md`: the messenger survey, research and the reasoning behind the design.
- `proto/`: Go reference implementation of the cryptography. `testdata/vectors.json` holds the test vectors the Android client must match.
- `server/`, `cmd/szeptd/`: the mailbox server (Go, SQLite).
- `android/`: Kotlin client.
  - `crypto/`: the protocol, tested against `vectors.json` (plain JVM).
  - `core/`: client logic and server API (plain JVM), tested against a real `szeptd`.
  - `app/`: the Android app (Compose).

## Server

```sh
go build -o szeptd ./cmd/szeptd

# first device: create an admin invite
./szeptd invite -data /var/lib/szept -url https://szept.home.lan

# run it, behind Caddy/nginx for TLS (or pass -tls-cert/-tls-key)
./szeptd serve -data /var/lib/szept -addr 127.0.0.1:8080 \
  -url https://szept.home.lan -push-allow https://ntfy.home.lan/ -trust-proxy
```

The server stores device and mailbox routing data, plus ciphertext that hasn't been delivered yet. A message is deleted as soon as the recipient acknowledges it, and after 7 days in any case. Files are deleted after their last download, and after 7 days in any case.

## Tests

```sh
go test ./...
(cd android && ./gradlew test)   # Kotlin crypto against the Go vectors
```

## Android app

```sh
cd android
./gradlew :app:assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

Debug builds allow plain `http://` servers for testing on the LAN; release builds need https.

Testing on the emulator without a second phone:

```sh
emulator -avd szept &                                   # AVD with system-images;android-35;default
./szeptd serve -data /tmp/sz -addr 127.0.0.1:8088 -url http://10.0.2.2:8088 &
adb reverse tcp:8088 tcp:8088                           # so the app reaches the bot's 127.0.0.1 URL
./szeptd invite -data /tmp/sz -url http://10.0.2.2:8088 # paste into the app's setup screen
# a scripted peer: registers, reads the app's QR from the screen, echoes every text
./gradlew :core:peerBot -PbotArgs="$(./szeptd invite -data /tmp/sz -url http://127.0.0.1:8088) /tmp/botqr.txt"
# then paste the contents of /tmp/botqr.txt into the app's Pair screen
```

After a protocol change, regenerate the vectors with `go test ./proto -run TestVectors -update`.
