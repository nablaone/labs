#pragma once

/*
 * BLE link: streams every DCC packet dcc.c receives to a connected
 * Bluetooth Low Energy client -- the web page in ../web/index.html,
 * via Chrome's Web Bluetooth. BLE rather than Wi-Fi so the viewing
 * laptop/phone keeps its normal internet connection.
 *
 * GATT layout (NimBLE peripheral, advertised as "DCC-Sniffer"):
 *   service  d0cc0001-6b2e-4c8a-9f3e-5a1b2c3d4e5f
 *     packets d0cc0002-6b2e-4c8a-9f3e-5a1b2c3d4e5f  (notify)
 *
 * Each notification carries one or more back-to-back records:
 *   u8  len          bytes of DCC data that follow (checksum included)
 *   u32 time_ms      little-endian, ms since ESP boot (packet end bit)
 *   u8  data[len]
 * Batched up to the negotiated MTU or BLE_BATCH_MS, whichever comes
 * first -- a busy layout sends a few hundred packets a second, too many
 * for one notification each.
 *
 * Only consumes dcc_receive() while a client is subscribed. While one
 * is, "dcc sniff" competes for the same packets (one queue, see dcc.h),
 * so don't run both at once.
 */

/* NVS + NimBLE host + GATT service + advertising. */
void ble_link_init(void);

/* Drains dcc_receive() into notifications while a client is subscribed. */
void ble_link_tx_task(void *arg);

/* Registers the "ble" CLI command (connection state and counters). */
void ble_link_register_cli_commands(void);
