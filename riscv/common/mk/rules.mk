# Build rules, run inside the riscv-lab container (IN_DOCKER=1).
# Included by the top Makefile; paths are relative to riscv/.

CROSS  := riscv64-unknown-elf-
BUILD  := build
RT     := common/sw/rt
SOC    := common/sim/soc.v
EXPS   := $(sort $(patsubst exp/%/,%,$(wildcard exp/*/)))

# ---- program build -------------------------------------------------------
# $(call prog_rules,<srcdir>,<outdir>,<march>)
# Builds <outdir>/prog.elf and <outdir>/prog.hex from all .c/.S in <srcdir>
# plus the common runtime. The hex is padded to the full 64 KB of RAM.
define prog_rules
$(2)/prog.elf: $(wildcard $(1)/*.c $(1)/*.S) $(wildcard $(RT)/*)
	@mkdir -p $(2)
	$(CROSS)gcc -march=$(3) -mabi=ilp32 -Os -g -ffreestanding -nostdlib -Wall \
		-I$(RT) -T $(RT)/link.ld $(RT)/crt0.S $(wildcard $(1)/*.c $(1)/*.S) \
		-Wl,--no-warn-rwx-segments -lgcc -o $$@
	$(CROSS)objdump -d $$@ > $(2)/prog.lst

$(2)/prog.hex: $(2)/prog.elf
	$(CROSS)objcopy -O binary --pad-to 0x80010000 $$< $(2)/prog.bin
	od -An -v -tx4 -w4 $(2)/prog.bin | tr -d ' ' > $$@
endef

# Where a program's sources live: experiment's own sw/ wins over common/sw/.
prog_src = $(firstword $(wildcard $(EXP)/sw/$(1) common/sw/$(1)))

# ---- experiment selection ------------------------------------------------
# (skipped for `new`, where E names an experiment that doesn't exist yet)
ifneq ($(E)$(filter new,$(MAKECMDGOALS)),$(E)new)
ifneq ($(E),)
EXP := $(firstword $(wildcard exp/$(E)*))
ifeq ($(EXP),)
$(error no experiment matching exp/$(E)*; try make list)
endif
EXP_MARCH := rv32i
EXP_DEFS  :=
include $(EXP)/exp.mk
EXPN   := $(notdir $(EXP))
EOUT   := $(BUILD)/$(EXPN)
PROGS  := $(if $(P),$(P),$(EXP_PROGS))
TBVVP  := $(EOUT)/tb.vvp

$(foreach p,$(PROGS),$(if $(call prog_src,$(p)),,$(error program $(p) not found in $(EXP)/sw or common/sw)))
$(foreach p,$(PROGS),$(eval $(call prog_rules,$(call prog_src,$(p)),$(EOUT)/$(p),$(EXP_MARCH))))

$(TBVVP): $(addprefix $(EXP)/,$(EXP_RTL) $(EXP_TB)) $(SOC)
	@mkdir -p $(EOUT)
	iverilog -g2012 $(EXP_DEFS) -s tb -o $@ $^

run: $(TBVVP) $(foreach p,$(PROGS),$(EOUT)/$(p)/prog.hex)
	@for p in $(PROGS); do \
		echo "== $(EXPN) / $$p"; \
		(cd $(EOUT)/$$p && vvp -n ../tb.vvp +hex=prog.hex $(SIMARGS)) \
			| grep -v '^VCD info\|\$$finish called'; \
	done

lint:
	verilator --lint-only -Wall -Wno-fatal -Wno-DECLFILENAME $(EXP_DEFS) \
		$(addprefix $(EXP)/,$(EXP_RTL))
else
run lint:
	$(error give an experiment, e.g. make $@ E=01)
endif
endif

# ---- QEMU reference (common programs only, plain rv32i) -----------------
ifneq ($(P),)
ifeq ($(E),)
$(eval $(call prog_rules,common/sw/$(P),$(BUILD)/qemu/$(P),rv32i))
endif
qemu: $(BUILD)/qemu/$(P)/prog.elf
	timeout 10 qemu-system-riscv32 -machine virt -bios none -nographic -kernel $<
else
qemu:
	$(error give a program, e.g. make qemu P=hello)
endif

# ---- across experiments --------------------------------------------------
list:
	@for e in $(EXPS); do \
		printf '%s  %s\n' "$$e" "$$(head -1 exp/$$e/README.md | sed 's/^# [^:]*: *//')"; \
	done

# One line per experiment x program, e.g.
#   01-femto-quark hello PASS 1177 cycles
compare:
	@for e in $(EXPS); do \
		n=$${e%%-*}; \
		$(MAKE) -s run E=$$n 2>&1 | awk -v e=$$e ' \
			/^== / { p=$$4 } \
			/^\[tb\] PASS/ { print e, p, "PASS", $$4, "cycles" } \
			/^\[tb\] ABORT/ { print e, p, "ABORT", $$7, "cycles" } \
			/^\[tb\] (FAIL|TIMEOUT)/ { print e, p, $$0 }'; \
	done

all: compare

# ---- scaffolding ---------------------------------------------------------
new:
	@test -n "$(E)" -a -n "$(FROM)" -a -n "$(NAME)" || \
		{ echo "usage: make new E=02 FROM=01 NAME=quark-mac"; exit 1; }
	@src=$$(ls -d exp/$(FROM)* | head -1); dst=exp/$(E)-$(NAME); \
	test -n "$$src" || { echo "no exp/$(FROM)*"; exit 1; }; \
	test ! -e $$dst || { echo "$$dst exists"; exit 1; }; \
	cp -r $$src $$dst; \
	printf '# %s: TODO one-line description\n\nDerived from: %s\n\nWhat changed:\n\nResults (make run E=%s):\n' \
		"$(E)-$(NAME)" "$$(basename $$src)" "$(E)" > $$dst/README.md; \
	sed -i "1s|.*|# Experiment $(E): derived from $$(basename $$src).|" $$dst/exp.mk; \
	echo "created $$dst from $$src"

clean:
	rm -rf $(BUILD)

.PHONY: run lint qemu list compare all new clean
