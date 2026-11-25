# RV32 toolchain for the riscv lab.
#   iverilog       Verilog compiler (iverilog) + simulator (vvp)
#   verilator      lint / fast C++ simulation later
#   gcc-riscv64-unknown-elf  cross GCC; builds rv32i/ilp32 with -march/-mabi
#   qemu-system-misc         provides qemu-system-riscv32
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        make \
        gcc g++ \
        gcc-riscv64-unknown-elf binutils-riscv64-unknown-elf \
        iverilog \
        verilator \
        qemu-system-misc \
    && rm -rf /var/lib/apt/lists/*

# The Makefile checks this to run rules directly instead of re-entering Docker.
ENV IN_DOCKER=1
WORKDIR /work
