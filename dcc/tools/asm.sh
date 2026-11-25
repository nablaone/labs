#!/bin/bash
# Disassembles the linked firmware (build/dcc.elf), keeping only the
# functions defined in main/*.c, one listing per module in asm/. The C
# source is interleaved as "# " comments. Runs inside the build
# container: make asm.
set -euo pipefail
OBJDIR=build/esp-idf/main/CMakeFiles/__idf_main.dir
ELF=build/dcc.elf
P=riscv32-esp-elf-
mkdir -p asm
full=$(mktemp)
# -d: disassemble code sections, -S: interleave source, -l: also print
# file:line markers, used below to tell our functions from same-named
# static functions elsewhere in ESP-IDF (then dropped). Paths in the
# debug info are /app/... inside the container, so strip that prefix.
${P}objdump -d -S -l --source-comment="# " "$ELF" | sed 's#/app/##g' > "$full"
for obj in "$OBJDIR"/*.c.obj; do
	m=$(basename "$obj" .c.obj)
	# Function symbols defined in this module (t = static, T = global).
	funcs=$(${P}nm --defined-only "$obj" | awk '$2 ~ /^[tT]$/ { print $3 }' | sort -u)
	{
		echo "# $m.c, disassembled from $ELF (ESP32-C3, rv32imc, -Og)."
		echo "# Regenerate with: make asm"
		echo "#"
		echo "# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction."
		echo
		# Collect each wanted function's block; print it only if its
		# file:line markers point at main/<module>.c.
		awk -v list="$funcs" -v src="main/$m.c:" '
			BEGIN { n = split(list, a, "\n"); for (i = 1; i <= n; i++) want[a[i]] = 1 }
			function flush() { if (on && mine) printf "%s", buf; buf = ""; mine = 0 }
			/^[0-9a-f]+ <[^>]+>:$/ {
				flush()
				name = $2; sub(/^</, "", name); sub(/>:$/, "", name)
				on = (name in want)
			}
			!on { next }
			index($0, src) == 1 { mine = 1 }
			/^[^ \t#]+\.[ch]:[0-9]+/ { next }     # -l file:line marker
			/^[A-Za-z_][A-Za-z0-9_.]*\(\):$/ { next } # -l function marker
			{ buf = buf $0 "\n" }
			END { flush() }
		' "$full"
	} > "asm/$m.s"
	echo "asm/$m.s: $(grep -c '^[0-9a-f]* <' "asm/$m.s") functions"
done
rm -f "$full"
