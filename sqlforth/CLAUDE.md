# CLAUDE.md

Guidance for AI assistants (and humans) working in this repository.

## What this is

**sqlforth** is a query language that is a *subset of classic ANS Forth* extended with
relational/SQL words. You write postfix Forth; data flows left-to-right through pipeline
stages the way it does in Google's Pipe SQL. The payoff over Pipe SQL is that stages are
just Forth words, so they are user-definable and composable (`: ... ;`).

Single binary, single file, zero external dependencies (Go stdlib only).

## Build / run

```sh
go run .                 # interactive REPL (reads stdin, prints "ok")
go run . script.fth      # file mode: run a program and exit
go vet ./...             # static checks
go build -o sqlforth .    # produce a binary
```

There is no test suite yet (see TASKS.md). To sanity-check a change, pipe a session:

```sh
printf '%s\n' '2 3 + .' 'from users col age 30 > where show' | go run .
```

The demo in `main()` writes a sample `people.csv` to the CWD on startup.

## Layout

Everything lives in `main.go` (~660 lines). Sections are marked with banner comments:

- value types (`VInt`, `VStr`, `Col`, `Lit`, `Binop`, `Query`, `Tabular`, …)
- the Forth machine (`Machine`, `Word`, `Instr`, `feed`, `execWord`)
- the dictionary (`newMachine` — every word is registered here)
- `where`/`select` polymorphism (`doWhere`, `doSelect`, `doShow`)
- the SQL compiler (`compile`, `fold`, `exprSQL`)
- the Tabular evaluator (`evalExpr`, `applyOp`)
- lexer (`tokenize`) and REPL (`repl`, `runLine`)

Keep the file single until it genuinely hurts; splitting prematurely obscures the small core.

## The mental model you must hold

Two ideas govern almost every change:

1. **Two relation types, one is lazy.** A `Query` is a deferred relational-algebra tree that
   compiles to SQL. A `Tabular` is materialized rows processed in Go. The transition
   `Query → Tabular` is the *pushdown boundary*. Words like `where`/`select` are polymorphic:
   on a `Query` they stay lazy (wrap a node); on a `Tabular` they execute immediately.

2. **STATE decides execute-vs-compile.** Outside a definition (`Interpret`), words run. Inside
   `: ... ;` (`Compile`), non-immediate words are *appended* to the definition's threaded code;
   `IMMEDIATE` words run anyway. This is real Forth semantics — do not break it.

If a change makes a word behave the same in both states, or makes a `Query` operation eagerly
touch data, stop and reconsider — you have probably broken an invariant.

## Invariants — do not regress these

- **Literals become SQL parameters, never string interpolation.** `exprSQL` emits `?` and
  collects args. Any new codegen must parameterize. This is the injection boundary.
- **Parsing words are `IMMEDIATE` and STATE-aware.** `col`, `from`, `csv`, and `S"` read from
  the token stream. In `Compile` state they must *bake* their result into the definition
  (via `emit`, which appends a `Lit`), not push to the runtime stack. `col`/`from` bake a
  static value; `csv` bakes `[filename literal] + [(csv) runtime word]` because the file must
  be opened at run time, not compile time. Follow whichever pattern fits.
- **Operator auto-lift.** `> < = and or` build an `Expr` node when either operand is an `Expr`
  (`isExpr`), otherwise fall back to classic Forth integer semantics (true = -1). Preserve both
  branches when touching `binop`.
- **Dictionary lookup is case-insensitive** (keys stored upper-cased). Column/table *names* are
  read raw via `nextToken`, so their case is preserved — never upper-case data.
- **The pipelined relation sits *below* the expression being built** on the stack. Operators
  only touch the top cells, so the relation is safe. Keep new expression-building words to the
  top of the stack.

## How to add things

- **A new pipeline stage** (`order`, `limit`, …): register a non-immediate word in `newMachine`
  that pops a relation and pushes a relation; implement both branches (Query → wrap a new
  algebra node + teach `fold`/`exprSQL`; Tabular → do it in Go). Add the node type near
  `Scan`/`Select`/`Project`.
- **A new expression operator**: add it to the `binop` loop and to `sqlOp`, `applyOp`.
- **A new core Forth word**: register a primitive in `newMachine`. Mark `immediate: true` only
  if it must act during compilation (control-flow words, parsing words).
- **A new SQL dialect**: today `compile` is dialect-agnostic. Introduce a `Dialect` on `Source`
  and thread it through `compile`/`exprSQL` (placeholder style, identifier quoting, `LIMIT`
  vs `TOP`). Pair it with `CanPush` so non-supported ops force materialization.

## Style

- Idiomatic Go, `gofmt`-clean, stdlib only unless a DB driver is explicitly added.
- Prefer small primitives composed via `:` definitions over large Go words, when the logic is
  expressible in the language itself.
- Error handling in the interpreter is panic/recover per REPL line; that is intentional for now.
