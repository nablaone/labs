# BLE throughput: why frames are slow

Notes from measuring the BLE diagnostic link (`main/ble_link.c`, protocol
in `main/ble_link.h`) on 2026-11-11. Board: FireBeetle 2 ESP32-S3,
ESP-IDF v5.4.2, NimBLE. Client: a MacBook (CoreBluetooth). Tests used a
Python `bleak` script (Chrome Web Bluetooth uses the same macOS stack).

**Short version:** the link carries about **10–30 KB/s**, or roughly
1–2.5 fps for QVGA JPEG. The firmware isn't the limit. The ESP's
controller has its transmit queue full almost all the time. The Mac, as
the BLE central, controls the connection interval and drains only a few
radio packets per connection event.

## What was negotiated

From the `ble:` log lines and the `ble` CLI command:

| parameter | value | notes |
|---|---|---|
| ATT MTU | 512 | `CONFIG_BT_NIMBLE_ATT_PREFERRED_MTU=512`. The Mac accepts it. |
| LL data length | 251 / 251 bytes | `ble_gap_set_data_len(conn, 251, 2120)` accepted |
| PHY | 2M | `ble_gap_set_prefered_le_phy()` accepted |
| connection interval | **30 ms** (sometimes 15 ms) | the Mac picks it. Our update requests mostly fail. |

So the radio settings are as good as BLE 5 gets. Only the interval is
poor, and the central decides that.

## Measurements

| setting | result |
|---|---|
| QVGA JPEG (~10–12 KB), 30 ms interval | 10–15 KB/s, ~1–1.4 fps |
| same, in runs that got 15 ms | ~30 KB/s, ~2.5 fps |
| 160×120 gray (19.2 KB) | 9–30 KB/s |
| 160×120 RGB565 (38.4 KB) | 9–14 KB/s, ~3–4 s per frame |

Throughput roughly **doubles when the interval halves**. That means a
fixed amount of data per connection event: about 450–900 bytes, or 2–4
link-layer packets. A 2M PHY could fit roughly 15–20 packets of 251
bytes into a 30 ms event.

## Where the bottleneck is

`ble_link.c` counts it, shown by the `ble` CLI command:

```
notifications  695
notify retries 5339 (controller full on 5339)
backed off     10678 ms
ctrl ACL free  12
```

Every retry happened while `ble_hs_hci_avail_pkts == 0`: the
controller's 12 ACL buffers were all waiting to go out on air. The
host side always had data ready. So:

- **Not the firmware.** The sender keeps the controller queue full.
- **Not NimBLE buffers.** The host only backs off because the
  controller is full.
- **The air link.** Few packets leave per connection event, and events
  are 30 ms apart.

In BLE the central controls how long each connection event lasts, and
the peripheral can only ask for more. macOS is known to cap packets per
event, and it also shares its 2.4 GHz radio with Wi-Fi (coexistence).

## Things tried that did not help

| attempt | result |
|---|---|
| connection interval 7.5–15 ms (`itvl_min=6, itvl_max=12`) | rejected: Apple's rules require min ≥ 15 ms and max ≥ min + 15 ms |
| 15–30 ms (`itvl_min=12, itvl_max=24`) | update fails with HCI 0x2a (*Different Transaction Collision*): it clashes with the PHY update and then the MTU exchange, even when retried once on failure. Some connections end up at 15 ms anyway. |
| back-off on `BLE_GAP_EVENT_NOTIFY_TX` semaphore vs. fixed 2 ms delay | no difference. The fixed delay is kept because it's simpler. NimBLE fires `NOTIFY_TX` when the notification is handed to the controller, not when it's sent over the air, so it says nothing about free buffers. |
| `CONFIG_BT_CTRL_CE_LENGTH_TYPE_SD` / `_CE` (controller connection-event length mode) | no difference (2.0–2.6 fps, same as ORIG). |
| `CONFIG_FREERTOS_HZ=1000` | needed so a 2 ms back-off is really 2 ms (at 100 Hz the shortest delay is 10 ms). Doesn't lift the cap. |

## Known bug: control writes can fail while streaming

During a stream, the client's `stream off` (or any ctrl write) can fail
with ATT error 0x11 *Insufficient Resources*. The sender piles
notifications into the NimBLE host until the msys mbuf pool is empty,
and then the ATT write response can't be allocated. It happens now and
then; in the web page it would show up as a failed Stop.

Two fixes were tried and backed out:

1. **Keep a reserve of free msys blocks**, using `os_msys_num_free()`.
   That function adds up *both* msys pools (32 × 256 B + 24 × 320 B =
   56 at idle). Everything we allocate comes from pool 1, so the
   reserve check passed while pool 1 was empty, and the bug stayed.
   Counting pool 2 out (`num_free - MSYS_2_BLOCK_COUNT`) made streaming
   stall completely. So pool 2 isn't idle during a stream, and the pool
   accounting is less simple than it looks.
2. **Only notify when the controller has free ACL credits**
   (`ble_hs_hci_avail_pkts >= packets needed + 1`), so nothing queues in
   the host. Written but never tested; reverted when BLE performance was
   put on hold.

Option 2 is the most promising next step if the bug starts to matter.

## If more speed is ever needed

Given the cap above, the options are, roughly in order of payoff:

1. **Send less.** Smaller frames, higher JPEG `q` (lower quality),
   grayscale crops, or only send results (bounding box, confidence)
   instead of images.
2. **Different central.** Android and Windows usually allow more packets
   per event and shorter intervals than macOS. Not measured yet.
3. **Wi-Fi.** The S3 has it. An HTTP MJPEG stream would easily do
   QVGA/VGA at camera speed (25–50 fps). The catch is network setup, or
   the laptop dropping its normal network if it joins a SoftAP. That's
   why the dcc project chose BLE.

## How to re-measure

- `ble` (USB CLI): MTU, interval, PHY, frames, notification/retry
  counters, controller ACL credits, msys free.
- The web page's info panel shows the BLE link (MTU, interval, PHY),
  and its header shows fps and KB/s.
- Frame transfer time per frame shows in the page caption (`… ms (… KB/s)`).
