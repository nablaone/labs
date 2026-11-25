// Testbench: FemtoRV32 Quark + 64 KB RAM + UART + exit device.
// Memory map matches QEMU virt, so the same ELF runs on both:
//   0x80000000  RAM, 64 KB, loaded from +hex=<file>
//   0x10000000  UART: a byte written here is printed
//   0x00100000  exit device (sifive_test encoding, see crt0.S)
`timescale 1ns/1ps

module tb;
    reg clk = 0;
    reg reset_n = 0;            // Quark's reset is active low
    always #5 clk = ~clk;       // 100 MHz, the period only matters for VCD

    wire [31:0] mem_addr, mem_wdata;
    wire [3:0]  mem_wmask;
    wire        mem_rstrb;
    reg  [31:0] mem_rdata;

    // ADDR_WIDTH=32: the default 24 would alias the UART at 0x10000000 to 0.
    FemtoRV32 #(.RESET_ADDR(32'h80000000), .ADDR_WIDTH(32)) cpu (
        .clk(clk),
        .mem_addr(mem_addr), .mem_wdata(mem_wdata), .mem_wmask(mem_wmask),
        .mem_rdata(mem_rdata), .mem_rstrb(mem_rstrb),
        .mem_rbusy(1'b0), .mem_wbusy(1'b0),
        .reset(reset_n)
    );

    localparam RAM_WORDS = 16384;
    reg [31:0] ram [0:RAM_WORDS-1];

    wire        is_ram  = mem_addr[31:16] == 16'h8000;
    wire        is_uart = mem_addr == 32'h10000000;
    wire        is_exit = mem_addr == 32'h00100000;
    wire [13:0] widx    = mem_addr[15:2];

    // Synchronous RAM: the core strobes mem_rstrb, data is there next cycle.
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

    integer cycle = 0;
    always @(posedge clk) begin
        cycle <= cycle + 1;
        if (cycle > 1_000_000) begin
            $display("[tb] TIMEOUT, pc stuck near 0x%08x", mem_addr);
            $finish;
        end
    end

    reg [1023:0] hexfile;
    integer i;
    initial begin
        for (i = 0; i < RAM_WORDS; i = i + 1) ram[i] = 32'h0;
        if (!$value$plusargs("hex=%s", hexfile)) begin
            $display("[tb] usage: vvp tb.vvp +hex=<file>");
            $finish;
        end
        $readmemh(hexfile, ram);
        if ($test$plusargs("vcd")) begin
            $dumpfile("tb.vcd");
            $dumpvars(0, tb);
        end
        repeat (4) @(posedge clk);
        reset_n = 1;
    end
endmodule
