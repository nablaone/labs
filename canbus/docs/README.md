# docs

Reference material for the loco/trackside control project, kept as markdown:
notes distilled from datasheets/PDFs, scrubbed excerpts from web pages,
protocol reference tables, design decisions, etc. One file per topic/
component. Cite sources (link + date) at the bottom of each file — these
notes are meant to stay useful after the original page/PDF is gone or
changed.

## Design record — start here

**[project-charter.md](project-charter.md)** is the keystone: what the
project is (a distributed control system for a rideable 7¼" gauge electric
locomotive, plus a modular trackside layout — this ESP32 lab is the
prototyping/learning vehicle for it), the platform decision, system shape,
and safety posture. Everything below is either linked from it or feeds it.

| Doc | Topic |
|---|---|
| [project-charter.md](project-charter.md) | Keystone: decisions, system shape, platform pivot, index |
| [bus-comparison.md](bus-comparison.md) | I2C/SPI/CAN/RS-232/RS-485/USB/Ethernet landscape |
| [bus-selection.md](bus-selection.md) | On-board I2C vs SPI detail |
| [canbus-vs-modbus.md](canbus-vs-modbus.md) | CAN vs Modbus/RS-485 |
| [can-message-spec.md](can-message-spec.md) | The message contract (IDs, priority, byte layout) |
| [esp-idf-architecture.md](esp-idf-architecture.md) | FreeRTOS concurrency, tasks, SMP, sync primitives |
| [zephyr-app-architecture.md](zephyr-app-architecture.md) | (historical) Zephyr RTOS model, pre-pivot |
| [wireless-architecture.md](wireless-architecture.md) | BLE/ESP-NOW/Thread/LoRa, safety boundaries |
| [trackside-control.md](trackside-control.md) | Trackside protocol, event model, learning concepts |
| [power-and-harness.md](power-and-harness.md) | 48V, Cat cable, Deutsch, buck tiers |
| [lighting-standard.md](lighting-standard.md) | Addressable LED standard (WS2812/RMT) |
| [hardware-packaging.md](hardware-packaging.md) | Enclosures, DIN, mounting |
| [diagnostics.md](diagnostics.md) | Debug/diagnose strategy |
| [can-components.md](can-components.md) | Ready-made CAN hardware |
| [reference-node.md](reference-node.md) | Node pattern: CAN out, I2C/Qwiic chain, GPIO |

Consolidated 2026-08-20/22; merged into this repo's `docs/` 2026-08-26 (was
drafted in a separate conversation, as `canbus-docs/`).

## Lab notes (this board, hands-on)

Narrower, board-specific notes from actually bringing up the ESP32 DevKitC
hardware in hand — feeds into the design record above rather than
duplicating it.

| Doc | Topic |
|---|---|
| [can-bus-bringup-plan.md](can-bus-bringup-plan.md) | TWAI pin plan, transceiver wiring, detection strategy for *this* board |
| [esp32-notes.md](esp32-notes.md) | This board's TWAI/flashing/FreeRTOS specifics |
| [dev-setup-research.md](dev-setup-research.md) | Dev-environment research (Docker, transceivers, macOS caveats) |
| [freertos-notes.md](freertos-notes.md) | FreeRTOS on ESP-IDF vs Zephyr, CAN-RX ISR pattern |
| [cli-toolchain.md](cli-toolchain.md) | (historical) STM32 CLI-toolchain research, pre-ESP32 pivot |
| [zephyr-single-app.md](zephyr-single-app.md) | (historical) shared Zephyr app design, pre-ESP-IDF pivot |
