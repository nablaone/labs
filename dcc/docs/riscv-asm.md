# Reading the firmware as RISC-V assembly

The ESP32-C3's CPU is a single 32-bit RISC-V core. This note goes through
the code in `main/` as the CPU actually sees it, using listings from the
real firmware.

```
make asm        # builds, then writes asm/<module>.s
```

`tools/asm.sh` disassembles the **linked** firmware (`build/dcc.elf`),
not the separate `.o` files. That way addresses are the real ones, calls
name their targets, and the linker's final changes ("relaxation", see
§4) are visible. Only functions from `main/*.c` are kept, with the C
source shown above each instruction group as `#` comments.

A listing line looks like this:

```
40381d22:	e02fe0ef          	jal	40380324 <esp_timer_get_time>
   │            │                    │
 address    encoding:               instruction
            4 hex digits = 2-byte compressed, 8 = 4-byte full size
```

The build uses `-Og` (optimise but keep it debuggable, the ESP-IDF
default). Each line of C turns into a recognisable group of
instructions, which makes it a good place to start. `-O2` code is
tighter but harder to match back to the source (§9).

---

## 1. Which RISC-V: `rv32imc`

RISC-V is a small base instruction set plus optional extensions. The
object files record which ones were used
(`Tag_RISCV_arch: rv32i2p1_m2p0_c2p0_zicsr…`):

| Letter | Meaning | Where it shows up |
|---|---|---|
| **RV32I** | the base: 32-bit integer registers, add/sub/logic/shift, load/store, branch, jump | everything |
| **M** | hardware multiply and divide (`mul`, `div`, `divu`, `rem`) | brightness scaling, histogram bucket |
| **C** | **compressed** 16-bit encodings of the most common instructions | 47% of our instructions |
| Zicsr | read/write control and status registers (interrupt enable, etc.) | inside FreeRTOS, not in our code |

There's no floating-point unit (no F/D). `float` maths is done in
software by library calls, which is one reason the firmware avoids it.

The **C** extension matters more than it looks. Our code has 1626
two-byte and 1834 four-byte instructions: 10.6 KB instead of 13.8 KB.
On this chip, code runs out of flash through a cache, so smaller code
also means fewer cache misses. A compressed instruction does exactly
the same as its full-size version. It just has fewer bits for register
numbers and constants.

## 2. Registers

There are 32 registers, `x0`–`x31`, each 32 bits. The disassembler
shows their **ABI names**, which tell you what each one is used for:

| Name | Reg | Role | Kept across a call? |
|---|---|---|---|
| `zero` | x0 | always reads 0, writes are thrown away | - |
| `ra` | x1 | return address, set by `jal` | no (save it if you call) |
| `sp` | x2 | stack pointer, 16-byte aligned | yes |
| `gp` | x3 | global pointer into the small-data area (§4) | fixed |
| `tp` | x4 | thread pointer | fixed |
| `a0`–`a1` | x10–11 | **arguments 1–2 and return value** (64-bit values use the pair) | no |
| `a2`–`a7` | x12–17 | arguments 3–8 | no |
| `t0`–`t6` | x5–7, x28–31 | temporaries | no |
| `s0`–`s11` | x8–9, x18–27 | "saved" registers | **yes**: a function that uses one must restore it |

There are no flags (no carry, zero or negative bits). A branch compares
two registers directly. Comparing with zero is free because `x0` is
always zero.

## 3. Where code lives: the address tells you

Our functions' addresses fall into two ranges:

```
0x4000_0000   mask ROM          esp_rom_delay_us() and other boot-ROM routines
0x4037_C000   IRAM              internal RAM for code: dcc_edge_isr, on_bit, probe_done ...
0x4200_0000   flash, via cache  everything else: build_frame, cmd_led, app_main ...
0x3FC8_0000   DRAM              data: leds[], stats, brightness ...
```

Look at the two kinds of function in `asm/leds.s`:

```
4200900e <build_frame>:     flash
40381e50 <probe_done>:      IRAM
```

`probe_done` is marked `IRAM_ATTR` in the C source, and that is the
whole difference. Flash is read through a cache, and while anything
writes to flash (NVS, an OTA update), the cache is switched off. An
interrupt handler that runs from flash during that time crashes the
chip. Interrupt-time code therefore goes in internal RAM. In `dcc.c`,
the edge ISR and every function it calls (`on_bit`, `pkt_end`,
`lose_sync`, `pkt_reset`) are marked the same way, and all of them sit
at `0x4038xxxx`.

## 4. The smallest functions: return values and addresses

```
420099fe <leds_count>:
420099fe:	4521        li	a0,8        # return value goes in a0
42009a00:	8082        ret             # jump back to the address in ra
```

`LEDS_COUNT` is a constant, so the function is just "load 8 into a0
and return". Both are 2-byte compressed instructions. Many of the
names are **pseudo-instructions**: easier names the disassembler shows
for real instructions.

| You see | Actually is |
|---|---|
| `li a0,8` | `addi a0, zero, 8` (0 + 8) |
| `mv a0,a2` | `addi a0, a2, 0` |
| `ret` | `jalr zero, 0(ra)` (jump to ra, don't save a return address) |
| `j label` | `jal zero, label` |
| `beqz a5,L` | `beq a5, zero, L` |
| `seqz a0,a0` | `sltiu a0, a0, 1` (1 if a0 < 1 unsigned, i.e. a0 == 0) |
| `zext.b a3,a3` | `andi a3, a3, 0xff` (keep the low byte: C's `(uint8_t)` cast) |

**Loading a global: why there's `lui` everywhere.** One instruction
holds at most a 12-bit constant. A full 32-bit address is therefore
built in two steps: `lui` ("load upper immediate") sets the top 20
bits, and the load or store adds the bottom 12:

```
42009d48 <leds_set_change_cb>:          # change_cb = cb;
42009d48:	3fc997b7   lui	a5,0x3fc99         # a5 = 0x3fc99000
42009d4c:	38a7a223   sw	a0,900(a5)         # store a0 at 0x3fc99000 + 900 = 0x3fc99384
42009d50:	8082       ret
```

The 12-bit part is **signed** (−2048…+2047), so addresses just below a
4 KB boundary get a negative offset, as in `build_frame` below:
`lui a5,0x3fc99; add a5,a5,-500` → `0x3fc98e0c` (`leds[]`).

**The linker's shortcut: `gp`.** Now compare this function:

```
42009a02 <leds_brightness>:
42009a02:	8501c503   lbu	a0,-1968(gp)       # 3fc95250 <brightness>
42009a06:	8082       ret
```

That's one load instead of `lui` + `lbu`. The `gp` register is set at
boot to the middle of a small-data area. Any variable within ±2 KB of
it is one 12-bit offset away. The compiler doesn't know final
addresses, so it always emits `lui` + `lbu`. The **linker** then
rewrites the pair when the variable lands close enough to `gp` (that's
"relaxation"). If you disassemble `leds.c.obj` before linking, you get
the two-instruction version with `R_RISCV_RELAX` notes on it. Only
variables that happen to land within ±2 KB of `gp` get the short form
(`brightness`, `pending_half`). The rest keep their `lui`: big arrays
like `leds[]` and `stats`, but also small ones like `change_cb` that
were placed further away.

## 5. A call and a stack frame: `probe_done`

The RMT driver calls this from its interrupt when a capture finishes
(`leds.c`). It forwards the symbol count to a queue:

```c
static bool IRAM_ATTR probe_done(rmt_channel_handle_t ch,
				 const rmt_rx_done_event_data_t *ev, void *arg)
{
	BaseType_t woken = pdFALSE;
	size_t n = ev->num_symbols;
	xQueueSendFromISR((QueueHandle_t)arg, &n, &woken);
	return woken == pdTRUE;
}
```

```
                                         # on entry: a0 = ch, a1 = ev, a2 = arg, ra = return address
40381e50:	1101       add	sp,sp,-32       # make a 32-byte stack frame (sp stays 16-byte aligned)
40381e52:	ce06       sw	ra,28(sp)       # we call a function, which overwrites ra: save it
40381e54:	8532       mv	a0,a2           # 1st argument of the call = arg (the queue)
40381e56:	c602       sw	zero,12(sp)     # woken = 0          woken lives at sp+12
40381e58:	41dc       lw	a5,4(a1)        # a5 = ev->num_symbols  (4 bytes into the struct)
40381e5a:	c43e       sw	a5,8(sp)        # n = a5             n lives at sp+8
40381e5c:	4681       li	a3,0            # 4th argument: queueSEND_TO_BACK
40381e5e:	0070       add	a2,sp,12        # 3rd argument: &woken
40381e60:	002c       add	a1,sp,8         # 2nd argument: &n
40381e62:	3f60b0ef   jal	4038d258 <xQueueGenericSendFromISR>   # ra = 40381e66, jump
40381e66:	4532       lw	a0,12(sp)       # a0 = woken (the callee wrote it via the pointer)
40381e68:	157d       add	a0,a0,-1        # a0 = woken - 1 ...
40381e6a:	00153513   seqz	a0,a0           # ... == 0 ?  -> a0 = (woken == 1), the return value
40381e6e:	40f2       lw	ra,28(sp)       # restore ra
40381e70:	6105       add	sp,sp,32        # drop the frame
40381e72:	8082       ret
```

Things to notice:

- **Arguments travel in `a0`, `a1`, `a2`, …** The incoming arguments
  are moved into place for the outgoing call. `arg` was the 3rd
  incoming argument and becomes the 1st outgoing one.
- **`n` and `woken` have to be in memory**, because the C code takes
  their address (`&n`, `&woken`). A register has no address. That's
  the only reason this function needs a stack frame beyond saving `ra`.
- `xQueueSendFromISR` is a **macro**. The real function,
  `xQueueGenericSendFromISR`, takes a 4th argument for "back or front
  of the queue", which is the `li a3,0`.
- `woken == pdTRUE` is computed **without a branch**: subtract 1, then
  "is it zero". Branches cost time on any pipeline, so the compiler
  avoids them for simple true/false results.
- `jal` writes the return address (the next instruction, `40381e66`)
  into `ra`. That's why `ra` has to be saved first.

## 6. A loop with maths: `build_frame`

```c
for (int i = 0; i < LEDS_COUNT; i++) {
	uint8_t r = 0, g = 0, b = 0;
	if (leds[i].on) {
		r = (leds[i].r * brightness + 127) / 255;
		...
	}
	uint8_t *p = frame + i * 3;
	p[0] = order_rgb ? r : g;
	...
}
```

The compiler doesn't lay out the loop in source order. The **test sits
at the bottom**, and the first instruction jumps straight to it:

```
4200900e:	4601       li	a2,0                   # i = 0               (i lives in a2, frame in a0)
42009010:	a80d       j	42009042               # go to the loop test
  ...
42009040:	0605       add	a2,a2,1                # i++
42009042:	479d       li	a5,7
42009044:	06c7c863   blt	a5,a2,420090b4         # if 7 < i: done (that's i < 8 turned around)
```

RISC-V branches only compare **two registers**: there's no "compare
with a constant". So the 7 is loaded into a register first, and
`i < 8` becomes "exit if 7 < i".

**Array indexing is just address arithmetic:**

```
42009048:	3fc997b7   lui	a5,0x3fc99
4200904c:	00261713   sll	a4,a2,0x2              # a4 = i * 4     (led_state_t is 4 bytes: on, r, g, b)
42009050:	e0c78793   add	a5,a5,-500             # a5 = &leds[0]  (0x3fc98e0c)
42009054:	97ba       add	a5,a5,a4               # a5 = &leds[i]
42009056:	0007c783   lbu	a5,0(a5)               # a5 = leds[i].on  (load byte, unsigned)
4200905a:	dfc5       beqz	a5,42009012            # off -> the "r = g = b = 0" block
```

`i * 3` for `p` needs no multiply either: `(i << 1) + i`.

```
42009018:	00161793   sll	a5,a2,0x1              # a5 = i * 2
4200901c:	97b2       add	a5,a5,a2               # a5 = i * 3
4200901e:	97aa       add	a5,a5,a0               # a5 = frame + i*3 = p
```

**The brightness scaling uses the M extension:**

```
4200906a:	00174683   lbu	a3,1(a4)               # leds[i].r
4200906e:	8501c783   lbu	a5,-1968(gp)           # brightness (gp-relative, §4)
42009072:	02f686b3   mul	a3,a3,a5               # r * brightness
42009076:	07f68693   add	a3,a3,127              # + 127 (round to nearest)
4200907a:	0ff00813   li	a6,255
4200907e:	0306c6b3   div	a3,a3,a6               # / 255   <- a real hardware divide
42009082:	0ff6f693   zext.b	a3,a3                  # (uint8_t)
```

At `-Og` the compiler emits a real `div`. A divide takes many clock
cycles on most cores, far more than an add. At `-O2` the compiler
replaces "divide by the constant 255" with a multiply by its binary
reciprocal plus a shift. It's the same trick as a fixed-point DSP
implementing "÷ 255" as "× 1/255" in Q-format. Here it doesn't matter:
8 LEDs × 3 colours = 24 divides per frame.

## 7. The hot path: `dcc_edge_isr`

This runs on every edge of the DCC signal, roughly every 58 µs during
a preamble. It's the most time-critical code in the firmware. All of
it is in `asm/dcc.s`. Here are the interesting parts.

**A 64-bit value on a 32-bit CPU:**

```c
int64_t now = esp_timer_get_time();
uint32_t dt = (uint32_t)(now - last_edge_us);
last_edge_us = now;
```
```
40381d22:	e02fe0ef   jal	40380324 <esp_timer_get_time>   # returns 64 bits: a0 = low, a1 = high
40381d26:	3fc997b7   lui	a5,0x3fc99
40381d2a:	3607a683   lw	a3,864(a5)             # a3 = low word of last_edge_us
40381d2e:	40d506b3   sub	a3,a0,a3               # dt = low(now) - low(last)
40381d32:	36a7a023   sw	a0,864(a5)             # last_edge_us = now, low word ...
40381d36:	36b7a223   sw	a1,868(a5)             # ... and high word, 4 bytes on
```

`dt` is cast to 32 bits, so the compiler only subtracts the **low
words**. The high words can't affect the result. The 64-bit store is
just two 32-bit stores. The low word wraps around every 2³² µs (about
71 minutes), and that's harmless too: unsigned subtraction across the
wrap still gives the right `dt`, the same way a free-running hardware
timer is read.

**`stats.edges++` is three instructions, not one:**

```
40381d42:	4398       lw	a4,0(a5)               # read
40381d44:	0705       add	a4,a4,1                # modify
40381d46:	c398       sw	a4,0(a5)               # write
```

RISC-V only does arithmetic in registers. Memory is touched only by
loads and stores. That's why a counter shared between an ISR and a
task needs care: an interrupt between the `lw` and the `sw` would lose
an update. Here only the ISR writes `stats`, so it's safe.

**The histogram bucket: divide, then clamp:**

```
40381d48:	47a9       li	a5,10
40381d4a:	02f6d7b3   divu	a5,a3,a5               # bucket = dt / 10   (u = unsigned)
40381d4e:	4751       li	a4,20
40381d50:	00f77363   bgeu	a4,a5,40381d56         # if 20 >= bucket: fine, skip the next line
40381d54:	47d1       li	a5,20                  # else bucket = 20 (the last bucket, "≥ 200 µs")
```

**A window comparator in two instructions.** This is the core of the
DCC decoder, classifying a half-bit by its length:

```c
if (dt >= HALF_ONE_MIN_US && dt <= HALF_ONE_MAX_US)      /* 35 .. 77 µs: a "1" half */
	half = 1;
else if (dt > HALF_ONE_MAX_US && dt <= HALF_ZERO_MAX_US) /* 78 .. 12000 µs: a "0" half */
	half = 0;
else
	lose_sync(...);
```
```
40381d68:	fdd68793   add	a5,a3,-35              # a5 = dt - 35
40381d6c:	02a00713   li	a4,42                  # 42 = 77 - 35, the window width
40381d70:	02f77163   bgeu	a4,a5,40381d92         # if (dt - 35) <= 42, unsigned -> half = 1
40381d74:	fb268693   add	a3,a3,-78              # a3 = dt - 78
40381d78:	678d       lui	a5,0x3                 # a5 = 0x3000 = 12288 ...
40381d7a:	e9278793   add	a5,a5,-366             # ... - 366 = 11922 = 12000 - 78
40381d7e:	00d7e463   bltu	a5,a3,40381d86         # if (dt - 78) > 11922, unsigned -> lose_sync
40381d82:	4401       li	s0,0                   # half = 0
```

The C code has four comparisons, but the machine code has **one
subtract and one unsigned compare per window**. The trick is the same
as shifting a window comparator's input so the window starts at 0.
Subtract the lower edge, and anything *below* the window goes negative.
As an **unsigned** number, negative becomes huge (`35 − 36 = 0xFFFFFFFF`),
so it fails the "≤ width" test as well. One compare checks both edges.

Two more things in this snippet:

- `12000 - 78` doesn't fit in 12 bits, so it's built as `lui` + `add`
  with a negative low part, like the addresses in §4.
- `half` lives in **`s0`**, a saved register. The function calls
  `lose_sync`, `on_bit` and `pkt_end` and still needs `half`
  afterwards. Any `a`/`t` register could be overwritten by those calls,
  but `s0` is guaranteed to survive. In exchange the ISR must save the
  caller's `s0` on entry and restore it on exit (`sw s0,24(sp)` /
  `lw s0,24(sp)`).

**Keeping `now` in registers:**

```
40381e28:	0070       add	a2,sp,12               # 3rd argument: &woken
40381e2a:	3da1       jal	40381c82 <pkt_end>     # pkt_end(now, &woken)
```

`pkt_end(int64_t now, BaseType_t *woken)`. A 64-bit first argument
takes `a0`+`a1`, and `now` is *still there* from the
`esp_timer_get_time()` call. Nothing on this path overwrote `a0`/`a1`,
so the compiler only has to set up `a2`. That `jal` is a 2-byte
compressed call (`c.jal`, which only exists on RV32), possible because
`pkt_end` is nearby.

**This isn't the real interrupt entry.** `dcc_edge_isr` ends with a
plain `ret`, like any function. The hardware interrupt lands in
ESP-IDF's vector code first. That code saves *all* registers (an
interrupt can arrive between any two instructions, so nothing may be
disturbed), works out which GPIO fired, and calls our handler as a
normal C function. Afterwards it restores everything and returns with
`mret`. `portYIELD_FROM_ISR()` at the end (`jal vPortYieldFromISR`)
asks that exit path to switch to a different task.

## 8. `switch`, and the cost of `-Og`: `on_bit`

```
40381b2e:	3fc997b7   lui	a5,0x3fc99
40381b32:	3547a783   lw	a5,852(a5)             # pkt_state
40381b36:	4705       li	a4,1
40381b38:	06e78663   beq	a5,a4,40381ba4         # case PKT_BYTE
40381b3c:	4709       li	a4,2
40381b3e:	0ee78463   beq	a5,a4,40381c26         # case PKT_SEPARATOR
40381b42:	e7fd       bnez	a5,40381c30            # not PKT_PREAMBLE either: out
```

With only three cases, the `switch` becomes a chain of compares. With
more cases close together, compilers build a jump table instead: an
array of addresses, then `lw` + `jr`.

Shifting a DCC bit into the byte, MSB first:

```
40381ba8:	34e74783   lbu	a5,846(a4)             # cur_byte
40381bac:	0786       sll	a5,a5,0x1              # << 1
40381bae:	8fc9       or	a5,a5,a0               # | bit       (bit is the argument, a0)
40381bb0:	0ff7f793   zext.b	a5,a5                  # (uint8_t)
40381bb4:	34f70723   sb	a5,846(a4)             # store back
```

Through all of `on_bit`, every global access starts with its own
`lui a5,0x3fc99`, even though the value is the same every time. That's
`-Og`: each statement is compiled on its own so a debugger can stop on
any line. At `-O2` the base address would be kept in a register, and
most of these globals would stay in registers until the function
returns.

## 9. Things to try

- **Find any function**: search `asm/*.s` for `<name>:`. Every `jal`
  shows its target's name, so you can follow a call chain
  (`app_main` → `leds_init` → `show_locked` → `build_frame`).
- **Far calls**: `auipc ra,…` + `jalr …(ra)` is a call to something
  beyond `jal`'s ±1 MB reach. For example, `send_frame_locked` (flash,
  `0x4200…`) calls `esp_rom_delay_us`, which lives in the chip's mask
  ROM at `0x40000050`. `auipc` adds the upper 20 bits to the PC, and
  `jalr` adds the lower 12.
- **See `-O2`**: set *Component config → Compiler options →
  Optimization Level → Optimize for performance (-O2)* in
  `idf.py menuconfig` (`make
  shell`, then `idf.py menuconfig`), and run `make asm` again. Compare
  `build_frame`: the `div` should turn into a `mul` by a magic number
  and a shift. Set it back afterwards, because `-O2` makes the debugger
  and backtraces less exact.
- **Look up any instruction**: the RISC-V *Unprivileged ISA* spec, chapter
  "RV32I Base Integer Instruction Set", has one table per instruction
  format. The "RISC-V Green Card" is a single page of all of them.
