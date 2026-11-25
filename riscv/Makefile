# Run from the host: every target re-runs itself inside the riscv-lab image.
#   make image   build the Docker image
#   make sw      cross-compile sw/hello for rv32i
#   make qemu    run hello on qemu-system-riscv32 (virt machine)
#   make sim     run the same hello.elf on FemtoRV32 Quark in Icarus Verilog
#                (make sim SIMARGS=+vcd writes build/tb.vcd)
#   make lint    verilator lint of the core
#   make shell   interactive shell in the image

IMAGE := riscv-lab

ifneq ($(IN_DOCKER),1)

image:
	docker build -t $(IMAGE) docker

shell:
	docker run --rm -it -v $(CURDIR):/work $(IMAGE) bash

all sw qemu sim lint clean %:
	docker run --rm -e MAKEFLAGS -v $(CURDIR):/work $(IMAGE) make $@

.PHONY: image shell all sw qemu sim lint clean

else

CROSS   := riscv64-unknown-elf-
ARCH    := -march=rv32i -mabi=ilp32
CFLAGS  := $(ARCH) -Os -g -ffreestanding -nostdlib -Wall
HELLO   := sw/hello
BUILD   := build

all: sw

sw: $(BUILD)/hello.elf $(BUILD)/hello.hex

$(BUILD)/hello.elf: $(HELLO)/crt0.S $(HELLO)/hello.c $(HELLO)/link.ld
	mkdir -p $(BUILD)
	$(CROSS)gcc $(CFLAGS) -T $(HELLO)/link.ld $(HELLO)/crt0.S $(HELLO)/hello.c -lgcc -o $@
	$(CROSS)objdump -d $@ > $(BUILD)/hello.lst
	$(CROSS)size $@

# Flat image starting at 0x80000000, one little-endian 32-bit word per line,
# which is what $readmemh in the testbench wants.
$(BUILD)/hello.hex: $(BUILD)/hello.elf
	$(CROSS)objcopy -O binary $< $(BUILD)/hello.bin
	od -An -v -tx4 -w4 $(BUILD)/hello.bin | tr -d ' ' > $@

qemu: $(BUILD)/hello.elf
	timeout 10 qemu-system-riscv32 -machine virt -bios none -nographic \
		-kernel $<

$(BUILD)/tb.vvp: sim/tb_femto.v vendor/femtorv32/femtorv32_quark.v
	mkdir -p $(BUILD)
	iverilog -g2012 -DBENCH -o $@ $^

sim: $(BUILD)/tb.vvp $(BUILD)/hello.hex
	cd $(BUILD) && vvp -n tb.vvp +hex=hello.hex $(SIMARGS)

lint:
	verilator --lint-only -Wall -Wno-fatal -DBENCH vendor/femtorv32/femtorv32_quark.v

clean:
	rm -rf $(BUILD)

.PHONY: all sw qemu sim lint clean

endif
