# TASKS.md

Roadmap and backlog. `[x]` done, `[ ]` open, `[~]` partial.

## Done

- [x] Outer/inner interpreter with `STATE` (interpret vs compile).
- [x] Colon definitions `:` … `;` compiling to threaded code (`[]Instr`).
- [x] `IMMEDIATE`; STATE-aware parsing words (`col`, `from`, `csv`, `S"`).
- [x] Case-insensitive dictionary; comments `\` and `( )`.
- [x] Core words: `DUP DROP SWAP OVER ROT + - * / . .S`.
- [x] Relational words: `from`, `csv`, `col`, `> < = and or`, `{`, `where`, `select`, `show`.
- [x] Lazy `Query` → parameterized SQL (`compile`/`fold`/`exprSQL`); args as `?` placeholders.
- [x] Eager `Tabular` filter/project in Go; CSV loader with numeric coercion.
- [x] Polymorphic `where`/`select` over `Query` vs `Tabular`.
- [x] REPL (stdin, `ok` prompt, per-line error recovery) + file mode.

## Milestone A — Forth completeness

Make it a fuller, more credible ANS subset.

- [ ] Control flow: `IF` / `ELSE` / `THEN`. Needs `BRANCH`/`0BRANCH` instructions with
      resolved addresses and a compile-time control stack.
- [ ] Loops: `BEGIN`/`UNTIL`/`WHILE`/`REPEAT`, then `DO`/`LOOP`/`+LOOP`.
- [ ] Return stack: `>R` `R>` `R@` (required for idiomatic loops and temporaries).
- [ ] Variables: `VARIABLE` + `@` `!`, and `CONSTANT`. Cells are `any`, so a variable can hold
      a `Query` or `Tabular` — i.e. **named relations / CTE-like views**:
      `from users adults base !   base @ pl show`.
- [ ] `VALUE` / `TO` (read without `@`) as the ergonomic variable form.
- [ ] `CREATE` … `DOES>` so variables/constants are defined *in the language*, not hardcoded.
- [ ] `.` / number output respecting `BASE`; add `U.` etc. (optional).

## Milestone B — Real query execution

Turn `Query` from "prints SQL" into "runs SQL".

- [x] Wire a `database/sql` driver. Pure-Go SQLite (`modernc.org/sqlite`) avoids cgo; note the
      module proxy may need configuring in restricted networks.
- [ ] `connect` word → open a DB, produce a `Source` with an open handle + dialect. Today the
      handle is opened once in `main()` against `sqlforth.db` and shared via the default `Source`.
- [x] `force()` on `Query`: compile → execute → read rows into a `Tabular`.
- [ ] `CanPush(pred/op)` per `Source`: if unsupported, `force()` then continue in Go. This
      activates the real pushdown boundary described in ARCHITECTURE.md.
- [~] `show`/`collect` route through `force()` when the plan roots in a DB source. `show` does;
      there is no `collect` word yet.
- [ ] Optional **Stream** state (open cursor) so pipelined ops don't buffer; only blocking ops
      materialize a full `Tabular`.

## Milestone C — More relational algebra

- [ ] `order` (sort keys via a sentinel list) + `asc`/`desc`.
- [ ] `limit` (and `offset`).
- [ ] `group` with aggregates (`count sum avg min max`). Design the two-list stack shape
      (keys + aggregates) — the awkward spot for postfix; consider two sentinel groups.
- [ ] `join` (`on <expr>`, kinds inner/left). Note: joining a `Query` with a `Tabular` forces
      materialization of one side; joining two DB `Query`s from the *same* source can push down.
- [ ] Extend the compiler beyond a linear `Scan→Select→Project` chain (subqueries / nested
      `SELECT` when projection or grouping intervenes).
- [ ] Auto-lift `+ - * /` to `Expr` for computed projections (`col a col b + as total`),
      mirroring the comparison operators.

## Milestone D — Dialects & I/O

- [ ] `Dialect` on `Source`: placeholder style (`?` vs `$1`), identifier quoting,
      `LIMIT` vs `TOP`. Target SQLite + Postgres first.
- [ ] Write-back: `to-csv <file>`, and `to-table <name>` (INSERT/CREATE). Enables the
      CSV↔SQL round-trip that motivated the project.
- [ ] Type-aware CSV loading (dates, floats, explicit schema override).

## Milestone E — Ergonomics & robustness

- [ ] Better errors: token position, stack-effect hints, "did you mean" on unknown words;
      keep the REPL alive on error (already partial).
- [ ] Multi-line `S"` / here-strings; `."` for output.
- [ ] `words` (list dictionary), `see <word>` (decompile a colon body).
- [ ] Float support (`VFloat`) end to end (lexer, arithmetic, SQL, eval).
- [ ] `.S` pretty-printing for `Query` (show the compiled SQL) and `Tabular` (row preview).

## Milestone F — Testing & tooling

- [ ] Golden tests: program text → expected stdout (table output and compiled SQL+args).
- [ ] Unit tests for `compile`/`exprSQL` (parameterization, WHERE composition) and `applyOp`
      (coercion, NULL/three-valued logic once NULL exists).
- [ ] Fuzz the tokenizer.
- [ ] Split `main.go` into packages (`forth`, `plan`, `sql`, `eval`) once it stops being small.
- [ ] `go.mod` housekeeping; CI (`go vet`, `go test`) on push.

## Known correctness risks to track

- [ ] **NULL / three-valued logic.** Once a real DB is connected, a predicate run in SQL vs the
      same predicate run in Go (post-`force`) must agree on NULL semantics. Decide and test.
- [ ] **Greedy materialization ordering.** Because `force()` triggers at the first non-pushable
      op, stage order changes what runs where. Document; consider a small reorder pass over the
      lazy prefix later.
- [ ] **Injection.** Every codegen path must parameterize literals. Guard with a test that fails
      if a literal ever appears inline in generated SQL.
