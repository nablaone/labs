# sqlforth

A query language that is a *subset of classic ANS Forth* extended with
relational/SQL words. You write postfix Forth; data flows left-to-right
through pipeline stages the way it does in Google's Pipe SQL. The payoff
over Pipe SQL is that stages are just Forth words, so they are
user-definable and composable (`: ... ;`).

Single binary, single file, zero external dependencies beyond the Go
standard library and a pure-Go sqlite driver.

See [`CLAUDE.md`](CLAUDE.md) for the contributor-facing mental model and
[`ARCHITECTURE.md`](ARCHITECTURE.md) for the design writeup. [`TASKS.md`](TASKS.md)
tracks the roadmap.

## Build / run

```sh
make build      # go build -o sqlforth .
make db         # (re)create sqlforth.db from schema.sql, from scratch
make sqlite     # open an interactive sqlite3 shell on sqlforth.db
make run        # build, then run the REPL
make vet        # go vet ./...
make clean      # remove the binary, the sqlite db, and the demo CSV
```

`sqlforth` opens `sqlforth.db` on startup (creating an empty file if it
doesn't exist yet — run `make db` first to get the seeded `users` table).
A demo `people.csv` is also written to the working directory on startup.

Run the REPL directly, or feed it a script:

```sh
./sqlforth                 # interactive REPL (reads stdin, prints "ok")
./sqlforth script.fth      # file mode: run a program and exit
```

## Examples

### Core Forth: stack and arithmetic

```
2 3 + .
5  ok

5 DUP + .
10  ok
```

### Querying the sqlite-backed `users` table (lazy `Query` → SQL)

`from` starts a lazy `Query` over a table. `where`/`select` on a `Query`
stay lazy — they wrap the relational-algebra tree instead of touching any
rows. `show` compiles it down to parameterized SQL and, since `sqlforth`
already has `sqlforth.db` open (see above), runs it there and prints the
resulting rows:

```
from users show
name | age | country
Anna | 34 | PL
Bob | 22 | US
Celine | 41 | PL
Dmitri | 29 | RU
 ok

from users col age 30 > where show
name | age | country
Anna | 34 | PL
Celine | 41 | PL
 ok

from users { col name col age select show
name | age
Anna | 34
Bob | 22
Celine | 41
Dmitri | 29
 ok
```

If a `Query`'s `Source` has no open DB handle, `show` instead falls back
to printing the compiled SQL and args (`[Query -> SQL] ...`) — this is the
path taken before `force()` had a real connection to execute against.

Note the `{ col ... col ... select` shape: `{` pushes a sentinel, each
`col <name>` pushes a column reference, and `select` pops back to the
sentinel to collect the projected columns.

### Querying a CSV file (eager `Tabular` → runs in Go)

`csv <file>` loads the file into memory immediately as a `Tabular`, so
`where`/`select` execute right away in Go instead of compiling to SQL:

```
csv people.csv col age 30 > where { col name col country select show
name | country
Anna | PL
Celine | PL
 ok
```

### Defining your own pipeline stage

Because pipeline stages are ordinary Forth words, you can name your own
and reuse them — this is the composability win over a fixed SQL dialect:

```
: adults col age 18 > where ;

from users adults show
name | age | country
Anna | 34 | PL
Bob | 22 | US
Celine | 41 | PL
Dmitri | 29 | RU
 ok

csv people.csv adults { col name select show
name
Anna
Bob
Celine
Dmitri
 ok
```

`adults` works unchanged over both a `Query` (stays lazy, compiles to SQL)
and a `Tabular` (runs immediately in Go) — that's the `where`/`select`
polymorphism described in `CLAUDE.md`.
