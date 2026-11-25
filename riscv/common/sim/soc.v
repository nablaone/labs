// The "lab bench" shared by all experiments: clock, reset, 64 KB RAM,
// a UART probe and an exit device, on a FemtoRV-style memory bus.
// Only the CPU is the device under test; everything here is simulation
// equipment ($write, $finish, # delays are not synthesizable).
//
// Memory map, matching QEMU virt so the same ELF runs on both:
//   0x80000000  RAM, 64 KB, loaded from +hex=<file>
//   0x10000000  UART: a byte written here is printed
//   0x00100000  exit device (sifive_test: 0x5555 pass, (code<<16)|0x3333 fail)
//
// Bus protocol (FemtoRV): the CPU pulses mem_rstrb with an address, the
// word is on mem_rdata the next cycle. A write is any cycle with a
// non-zero mem_wmask (one bit per byte lane). A CPU with a different bus
// needs an adapter in its experiment's sim/ directory.
//
// Plusargs: +hex=<file> (required), +vcd (dump tb.vcd), +maxcycles=<n>.
`timescale 1ns/1ps

module soc (
    output reg        clk,
    output reg        reset_n,     // active low, released after 4 cycles
    input      [31:0] mem_addr,
    input      [31:0] mem_wdata,
    input      [3:0]  mem_wmask,
    input             mem_rstrb,
    output reg [31:0] mem_rdata,
    output            mem_rbusy,
    output            mem_wbusy
);
    assign mem_rbusy = 1'b0;   // RAM answers in one cycle, never busy
    assign mem_wbusy = 1'b0;

    initial clk = 0;
    always #5 clk = ~clk;      // 100 MHz; the period only matters for VCD

    localparam RAM_WORDS = 16384;
    reg [31:0] ram [0:RAM_WORDS-1];

    wire        is_ram  = mem_addr[31:16] == 16'h8000;
    wire        is_uart = mem_addr == 32'h10000000;
    wire        is_exit = mem_addr == 32'h00100000;
    wire [13:0] widx    = mem_addr[15:2];

    integer cycle = 0;
    integer maxcycles = 1_000_000;

    always @(posedge clk) begin
        if (mem_rstrb)
            mem_rdata <= is_ram ? ram[widx] : 32'h0;

        if (|mem_wmask) begin
            if (is_ram) begin
                if (mem_wmask[0]) ram[widx][ 7: 0] <= mem_wdata[ 7: 0];
                if (mem_wmask[1]) ram[widx][15: 8] <= mem_wdata[15: 8];
                if (mem_wmask[2]) ram[widx][23:16] <= mem_wdata[23:16];
                if (mem_wmask[3]) ram[widx][31:24] <= mem_wdata[31:24];
            end else if (is_uart) begin
                $write("%c", mem_wdata[7:0]);
                $fflush;
            end else if (is_exit) begin
                if (mem_wdata[15:0] == 16'h5555)
                    $display("[tb] PASS after %0d cycles", cycle);
                else
                    $display("[tb] FAIL exit code %0d after %0d cycles",
                             mem_wdata[31:16], cycle);
                $finish;
            end else begin
                $display("[tb] write to unmapped 0x%08x", mem_addr);
            end
        end
    end

    always @(posedge clk) begin
        cycle <= cycle + 1;
        if (cycle > maxcycles) begin
            $display("[tb] TIMEOUT after %0d cycles, addr 0x%08x",
                     cycle, mem_addr);
            $finish;
        end
    end

    reg [1023:0] hexfile;
    initial begin
        reset_n = 0;
        if (!$value$plusargs("hex=%s", hexfile)) begin
            $display("[tb] usage: vvp tb.vvp +hex=<file>");
            $finish;
        end
        if ($value$plusargs("maxcycles=%d", maxcycles)) ;
        // The hex file is padded to the full 64 KB, so this fills all RAM.
        $readmemh(hexfile, ram);
        if ($test$plusargs("vcd")) begin
            $dumpfile("tb.vcd");
            $dumpvars(0);
        end
        repeat (4) @(posedge clk);
        reset_n <= 1;   // non-blocking: changes after this edge, no race with the CPU
    end
endmodule
