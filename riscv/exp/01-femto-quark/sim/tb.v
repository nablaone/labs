// Experiment 01: unmodified FemtoRV32 Quark on the common lab bench.
`timescale 1ns/1ps

module tb;
    wire        clk, reset_n;
    wire [31:0] mem_addr, mem_wdata, mem_rdata;
    wire [3:0]  mem_wmask;
    wire        mem_rstrb, mem_rbusy, mem_wbusy;

    soc bench (
        .clk(clk), .reset_n(reset_n),
        .mem_addr(mem_addr), .mem_wdata(mem_wdata), .mem_wmask(mem_wmask),
        .mem_rstrb(mem_rstrb), .mem_rdata(mem_rdata),
        .mem_rbusy(mem_rbusy), .mem_wbusy(mem_wbusy)
    );

    // ADDR_WIDTH=32: the default 24 would alias 0x10000000 and 0x80000000 to 0.
    FemtoRV32 #(.RESET_ADDR(32'h80000000), .ADDR_WIDTH(32)) cpu (
        .clk(clk), .reset(reset_n),
        .mem_addr(mem_addr), .mem_wdata(mem_wdata), .mem_wmask(mem_wmask),
        .mem_rdata(mem_rdata), .mem_rstrb(mem_rstrb),
        .mem_rbusy(mem_rbusy), .mem_wbusy(mem_wbusy)
    );
endmodule
