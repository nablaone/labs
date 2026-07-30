# ARCHITECTURE.md

## Motivation

The language grew out of experiments with Google's **Pipe SQL**, where `|>` chains stages, each
of which takes a table and returns a table — a Unix-pipe feel for relational data. In a
**concatenative** language (Forth, Factor) the pipe operator disappears entirely: juxtaposing
two words *is* function composition, so plain adjacency already means "pipe". sqlforth leans into
that: it is a small ANS-Forth subset whose extra words operate on tables, so a query reads as a
sequence of stages, and any user-defined word with stack effect `( table -- table )` is a
first-class stage — indistinguishable from the built-ins. That openness (a programmable,
extensible stage set) is the reason to build this rather than just use Pipe SQL.

## Two layers

sqlforth is two things sharing one dictionary and one stack:

1. a **classic Forth engine** (outer/inner interpreter, `STATE`, colon definitions), and
2. a **relational layer** (a lazy query planner + an eager in-memory engine).

The seam between them is deliberately thin: relational values are just ordinary things on the
Forth stack, and relational words are just ordinary dictionary entries.

## The Forth engine

### Outer interpreter (`feed`)

Tokenizes a line and walks tokens. For each token:

- string literal (produced by the lexer for `S" … "`) → `emit`
- integer → `emit`
- otherwise look up in the dictionary (case-insensitive); unknown → error

`emit` is the STATE-sensitive primitive: in `Interpret` it pushes onto the runtime stack; in
`Compile` it appends a `Lit` instruction to the definition under construction.

### STATE and colon definitions

`STATE` is either `Interpret` or `Compile`.

- `:` reads the next token as a name, starts a fresh `Word`, and switches to `Compile`.
- In `Compile`, a non-immediate word found by `feed` is **appended** as an `Instr{Word: w}`
  rather than executed; numbers/strings are appended as `Instr{Lit: v}`.
- `IMMEDIATE` words execute even in `Compile`. `;` is immediate: it finalizes the current word
  into the dictionary and returns to `Interpret`.

This execute-vs-compile distinction — not the postfix syntax — is what makes sqlforth a genuine
Forth subset. `STATE` persists across REPL lines, so multi-line definitions work naturally.

### Inner interpreter (`execWord`)

A `Word` is either a **primitive** (`prim func(*Machine)`, written in Go) or a **colon word**
(a `body []Instr` of threaded code). Executing a colon word walks its instructions, executing
sub-words (recursively) and pushing literals. Classic ANS Forth uses an explicit instruction
pointer plus a return stack; host-language recursion is a semantically equivalent
implementation for this subset. The main consequence: control flow that needs branching
(`IF`/`THEN`, loops) is *not* free — it requires branch instructions with addresses and is
listed as future work.

### Parsing words

Forth has no quotations, but it has something arguably stronger: words that consume the input
stream (like standard `."` and `S"`). sqlforth uses this for syntax:

- `from <table>` reads the next token as a table name.
- `col <name>` reads the next token as a column name.
- `csv <file>` reads the next token as a path.

These are `IMMEDIATE` and STATE-aware. In `Interpret` they act immediately. In `Compile` they
must **bake** their effect into the definition:

- `col`/`from` bake a static `Lit` (the column ref / the `Query`), because the value is known at
  compile time.
- `csv` bakes `[Lit filename] + [Word (csv)]`, deferring the actual file open to run time via
  the hidden runtime word `(csv)`.

This mirrors how standard `S"` is state-smart, and is the classic-Forth answer to "structured
arguments without quotations".

### Sentinel lists

`where` consumes exactly one expression, but `select` needs a *list* of columns. The classic
idiom is a stack marker: `{` pushes a `sentinel{}`, and `select` pops columns until it hits the
marker. So `{ col name col email select` collects `[name, email]`.

### Core wordset (subset)

Registered in `newMachine`: `DUP DROP SWAP OVER ROT`, `+ - * /`, `. .S`, `: ;`, `IMMEDIATE`,
comments `\` and `( )`. Lookup is case-insensitive (`gforth`-style). This is a small,
recognizable slice of the ANS CORE word set.

## The relational layer

### Stack values

The stack is `[]any`. Scalars are `VInt` (int64) and `VStr`. Beyond scalars, three families of
structured values can sit on the stack:

- **Expr** — `Col{Name}`, `Lit{V}`, `Binop{Op,L,R}`: a predicate/expression tree.
- **Query** — a lazy relational-algebra tree (`Scan`, `Select`, `Project`) plus its `Source`.
- **Tabular** — materialized `Cols []string` + `Rows [][]any`.

`isExpr` distinguishes expression nodes from scalars, which drives operator auto-lift.

### Operator auto-lift (`binop`)

`> < = and or` inspect their two operands. If either is an `Expr`, they lift the other scalar to
a `Lit` and build a `Binop` node (deferred). Otherwise they behave as classic Forth integer
operators (result `-1`/`0`). One set of operators, two behaviors, chosen by whether an `Expr` is
in play. While a predicate is being built, the pipelined relation rests safely *underneath* it
on the stack.

### Laziness and the pushdown boundary

This is the core design decision. A `Query` is lazy and compiles to SQL; a `Tabular` is
materialized and computed in Go. Relational words are **polymorphic over the relation type**:

- `doWhere` / `doSelect` on a `Query` wrap a new algebra node and stay lazy.
- The same words on a `Tabular` execute immediately (filter rows / project columns in Go).

The transition `Query → Tabular` is the pushdown boundary — expressed in the type system rather
than as an optimizer pass. In the full design a word checks `Source.CanPush(pred)`: if the
backend can express the operation it stays a `Query`; otherwise it calls `force()` to materialize
and continues in Go. Because materialization is greedy (at the first non-pushable op), operator
order matters and the programmer is responsible for putting pushable stages first — which suits
the Forth spirit. *Current status:* `CanPush` is assumed true and `force()` is not yet wired in
(see TASKS.md); `Query` operations are always kept lazy.

A likely refinement is a third state, **Stream** (an open cursor), so that pipelined ops
(filter/project/limit) run one row at a time without buffering, and only *blocking* ops
(sort/join/group) materialize a full `Tabular`.

### SQL compiler (`compile` / `fold` / `exprSQL`)

`fold` walks the linear `Scan → Select → Project` chain into a `sqlParts` accumulator
(table, projected columns, WHERE fragments, args). Multiple `Select` nodes become WHERE
fragments joined with `AND`. `exprSQL` renders an `Expr`: columns inline, **literals as `?`
placeholders with args collected separately** (the injection boundary), binops parenthesized.
`compile` assembles `SELECT … FROM … [WHERE …]` and returns `(sql, args)`.

The compiler is currently dialect-agnostic and assumes a linear plan; join/group/order/limit and
per-dialect codegen are future work.

### Tabular evaluator (`evalExpr` / `applyOp`)

For the materialized path, `evalExpr` walks an `Expr` against a single row: `Col` resolves by
column index, `Lit` returns its value, `Binop` recurses. `applyOp` implements comparison and
boolean logic, with `coerce` normalizing operands (CSV cells are parsed to `VInt` when numeric,
else `VStr`). The same evaluator serves both `Tabular` filtering and any future post-`force`
computation.

## Data flow, end to end

```
source text
  → tokenize            (S" … " folded into one string-literal token)
  → feed (outer)        STATE: execute or compile into a Word body
      • Interpret:      execWord runs primitives / walks colon bodies
      • Compile:        append Instr{Word|Lit} to the current definition
  → relational words build a Query (lazy) or transform a Tabular (eager)
  → show:
      • Query    → compile → parameterized SQL (destined for a DB)
      • Tabular → print rows computed in Go
```

## Deliberate simplifications (today)

- No control flow (`IF/ELSE/THEN`, `BEGIN/UNTIL`, `DO/LOOP`) and no return stack (`>R`/`R>`).
- No real database execution: `show` prints the compiled SQL instead of running it.
- No `force()`/`CanPush`; `Query` never materializes.
- Only `Scan/Select/Project`; no join/group/order/limit; single-source plans only.
- `+ - * /` are integer-only and do **not** auto-lift to `Expr` (only comparisons/booleans do).
- Numbers are `int64` only (no floats); `S"` is single-line.
- Minimal error handling (panic/recover per REPL line).

These are scoped omissions, not accidents; each maps to a task in TASKS.md.
