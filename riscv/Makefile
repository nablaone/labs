# riscv lab. Run everything from the host; targets re-run inside Docker.
#
#   make image                         build the riscv-lab Docker image
#   make shell                         interactive shell in the image
#   make list                          list experiments
#   make run E=01                      simulate all programs of experiment 01
#   make run E=01 P=hello              one program
#   make run E=01 P=hello SIMARGS=+vcd also dump build/<exp>/<prog>/tb.vcd
#   make lint E=01                     verilator lint of the experiment's RTL
#   make qemu P=hello                  reference run on qemu-system-riscv32
#   make compare                       every experiment x program, cycle counts
#   make new E=02 FROM=01 NAME=quark-mac   copy an experiment to start a variant
#   make clean
#
# E matches the experiment number prefix (E=01 -> exp/01-*).

IMAGE := riscv-lab

ifneq ($(IN_DOCKER),1)

image:
	docker build -t $(IMAGE) docker

shell:
	docker run --rm -it -v $(CURDIR):/work $(IMAGE) bash

TARGETS := all list run lint qemu compare new clean
$(TARGETS):
	docker run --rm -e MAKEFLAGS -v $(CURDIR):/work $(IMAGE) make $@

.PHONY: image shell $(TARGETS)

else
include common/mk/rules.mk
endif
