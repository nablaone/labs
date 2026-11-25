// Experiment 02: FemtoRV32 Quark + custom 'puts' and 'abort' on the
// common lab bench. Same wiring as 01, plus a watcher on the new
// 'halted' pin, which ends the simulation after 'abort'.
`timescale 1ns/1ps

module tb;
    wire        clk, reset_n;
    wire [31:0] mem_addr, mem_wdata, mem_rdata;
    wire [3:0]  mem_wmask;
    wire        mem_rstrb, mem_rbusy, mem_wbusy;
    wire        halted;

    soc bench (
        .clk(clk), .reset_n(reset_n),
        .mem_addr(mem_addr), .mem_wdata(mem_wdata), .mem_wmask(mem_wmask),
        .mem_rstrb(mem_rstrb), .mem_rdata(mem_rdata),
        .mem_rbusy(mem_rbusy), .mem_wbusy(mem_wbusy)
    );

    // ADDR_WIDTH=32: the default 24 would alias 0x10000000 and 0x80000000 to 0.
    FemtoRV32 #(.RESET_ADDR(32'h80000000), .ADDR_WIDTH(32),
                .PUTS_ADDR(32'h10000000)) cpu (
        .clk(clk), .reset(reset_n),
        .mem_addr(mem_addr), .mem_wdata(mem_wdata), .mem_wmask(mem_wmask),
        .mem_rdata(mem_rdata), .mem_rstrb(mem_rstrb),
        .mem_rbusy(mem_rbusy), .mem_wbusy(mem_wbusy),
        .halted(halted)
    );

    // In hardware 'halted' could drive an LED or gate the clock. Here it
    // plays the role of the exit device, but the stop comes from the core.
    always @(posedge clk)
        if (halted) begin
            $display("[tb] ABORT at pc 0x%08x after %0d cycles",
                     cpu.PC - 4, bench.cycle);
            $finish;
        end
endmodule
