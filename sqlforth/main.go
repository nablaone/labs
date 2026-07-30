// sqlforth — a hybrid: a SUBSET of classic ANS Forth + a relational/SQL extension.
//
// What makes this a subset of ANS Forth (not just "Forth-like"):
//   - a text-based outer interpreter with a STATE distinction: interpret vs compile,
//   - colon definitions ":" ... ";" compile to threaded code ([]Instr),
//   - IMMEDIATE words execute even in compile mode,
//   - parsing words (col, from, csv, S") are IMMEDIATE and STATE-aware:
//       interpret -> run right away, compile -> bake the action into the definition,
//   - a case-insensitive dictionary, standard core words, comments \ and ( ),
//   - the classic "ok" in the REPL.
//
// SQL extension:
//   - Query   = a lazy relational algebra -> compiles to parameterized SQL,
//   - Tabular = a materialized table -> operations run in Go,
//   - the same words (where/select) behave polymorphically by relation type.
package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// name of the sqlite database file opened on startup (see: `make db`)
const dbFile = "sqlforth.db"

// ========================= stack values =========================

type VInt int64
type VStr string
type sentinel struct{}

type Col struct{ Name string }
type Lit struct{ V any }
type Binop struct {
	Op   string
	L, R any
}

func isExpr(v any) bool {
	switch v.(type) {
	case Col, Lit, Binop:
		return true
	}
	return false
}

type Source struct {
	Name, Dialect string
	DB            *sql.DB
}
type Query struct {
	Node any
	Src  *Source
}
type Scan struct{ Table string }
type Select struct {
	In   any
	Pred any
}
type Project struct {
	In   any
	Cols []string
}
type Tabular struct {
	Cols []string
	Rows [][]any
}

// ========================= Forth machine =========================

type State int

const (
	Interpret State = iota
	Compile
)

// threaded-code instruction: either a word call or pushing a literal
type Instr struct {
	Word *Word
	Lit  any
}

type Word struct {
	name      string
	prim      func(*Machine) // primitive (nil for a colon definition)
	body      []Instr        // body of a colon definition
	immediate bool
}

type Machine struct {
	stack []any
	toks  []string
	pos   int
	dict  map[string]*Word
	state State
	cur   *Word // definition currently being built
	last  *Word // most recently defined word (for IMMEDIATE)
	src   *Source
}

func (m *Machine) push(v any) { m.stack = append(m.stack, v) }
func (m *Machine) pop() any {
	if len(m.stack) == 0 {
		panic("stack underflow")
	}
	v := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return v
}
func (m *Machine) nextToken() string {
	if m.pos >= len(m.toks) {
		panic("expected a token after a parsing word")
	}
	t := m.toks[m.pos]
	m.pos++
	return t
}
func (m *Machine) lookup(name string) *Word { return m.dict[strings.ToUpper(name)] }
func (m *Machine) def(name string, immediate bool, prim func(*Machine)) {
	m.dict[strings.ToUpper(name)] = &Word{name: name, prim: prim, immediate: immediate}
}

// emit: literal -> in compile mode append to the definition, in interpret push to the stack
func (m *Machine) emit(lit any) {
	if m.state == Compile {
		m.cur.body = append(m.cur.body, Instr{Lit: lit})
	} else {
		m.push(lit)
	}
}

// inner interpreter (recursive; ANS uses IP+return stack, same semantics)
func (m *Machine) execWord(w *Word) {
	if w.prim != nil {
		w.prim(m)
		return
	}
	for _, in := range w.body {
		if in.Word != nil {
			m.execWord(in.Word)
		} else {
			m.push(in.Lit)
		}
	}
}

// outer interpreter: loop over tokens with a STATE distinction
func (m *Machine) feed() {
	for m.pos < len(m.toks) {
		t := m.toks[m.pos]
		m.pos++
		if strings.HasPrefix(t, "\x00") { // string literal (from S")
			m.emit(VStr(t[1:]))
			continue
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			m.emit(VInt(n))
			continue
		}
		w := m.lookup(t)
		if w == nil {
			panic("unknown word: " + t)
		}
		if m.state == Compile && !w.immediate {
			m.cur.body = append(m.cur.body, Instr{Word: w})
		} else {
			m.execWord(w)
		}
	}
}

// ========================= dictionary =========================

func newMachine() *Machine {
	m := &Machine{
		dict: map[string]*Word{},
		src:  &Source{Name: "main", Dialect: "sqlite"},
	}

	// --- word definition (ANS core) ---
	m.def(":", false, func(m *Machine) {
		m.cur = &Word{name: m.nextToken()}
		m.state = Compile
	})
	m.def(";", true, func(m *Machine) { // IMMEDIATE
		m.dict[strings.ToUpper(m.cur.name)] = m.cur
		m.last = m.cur
		m.cur = nil
		m.state = Interpret
	})
	m.def("IMMEDIATE", false, func(m *Machine) {
		if m.last != nil {
			m.last.immediate = true
		}
	})

	// --- comments ---
	m.def("\\", true, func(m *Machine) { m.pos = len(m.toks) }) // to end of line
	m.def("(", true, func(m *Machine) { // through ")"
		for m.pos < len(m.toks) && m.toks[m.pos] != ")" {
			m.pos++
		}
		if m.pos < len(m.toks) {
			m.pos++
		}
	})

	// --- stack (ANS core) ---
	m.def("DUP", false, func(m *Machine) { v := m.pop(); m.push(v); m.push(v) })
	m.def("DROP", false, func(m *Machine) { m.pop() })
	m.def("SWAP", false, func(m *Machine) { a := m.pop(); b := m.pop(); m.push(a); m.push(b) })
	m.def("OVER", false, func(m *Machine) { a := m.pop(); b := m.pop(); m.push(b); m.push(a); m.push(b) })
	m.def("ROT", false, func(m *Machine) { c := m.pop(); b := m.pop(); a := m.pop(); m.push(b); m.push(c); m.push(a) })

	// --- arithmetic and output (ANS core) ---
	m.def("+", false, func(m *Machine) { b := int64(m.pop().(VInt)); a := int64(m.pop().(VInt)); m.push(VInt(a + b)) })
	m.def("-", false, func(m *Machine) { b := int64(m.pop().(VInt)); a := int64(m.pop().(VInt)); m.push(VInt(a - b)) })
	m.def("*", false, func(m *Machine) { b := int64(m.pop().(VInt)); a := int64(m.pop().(VInt)); m.push(VInt(a * b)) })
	m.def("/", false, func(m *Machine) { b := int64(m.pop().(VInt)); a := int64(m.pop().(VInt)); m.push(VInt(a / b)) })
	m.def(".", false, func(m *Machine) { fmt.Print(valueStr(m.pop()), " ") })
	m.def(".S", false, func(m *Machine) {
		fmt.Printf("<%d> ", len(m.stack))
		for _, v := range m.stack {
			fmt.Print(valueStr(v), " ")
		}
		fmt.Println()
	})

	// --- SQL extension: sources (parsing words, IMMEDIATE + STATE-aware) ---
	m.def("from", true, func(m *Machine) { // from <table>
		tbl := m.nextToken()
		m.emit(Query{Node: Scan{Table: tbl}, Src: m.src})
	})
	m.def("csv", true, func(m *Machine) { // csv <file> — loading DEFERRED to runtime
		fn := m.nextToken()
		if m.state == Compile {
			m.cur.body = append(m.cur.body, Instr{Lit: VStr(fn)})
			m.cur.body = append(m.cur.body, Instr{Word: m.lookup("(csv)")})
		} else {
			m.push(loadCSV(fn))
		}
	})
	m.def("(csv)", false, func(m *Machine) { m.push(loadCSV(string(m.pop().(VStr)))) })

	// --- expression building ---
	m.def("col", true, func(m *Machine) { m.emit(Col{Name: m.nextToken()}) }) // col <column>
	for _, op := range []string{">", "<", "=", "and", "or"} {
		op := op
		m.def(op, false, func(m *Machine) { m.binop(op) })
	}

	// --- pipeline stages (table -- table) ---
	m.def("{", false, func(m *Machine) { m.push(sentinel{}) })
	m.def("where", false, func(m *Machine) {
		pred := m.pop()
		m.push(doWhere(m.pop(), pred))
	})
	m.def("select", false, func(m *Machine) {
		var cols []string
		for {
			v := m.pop()
			if _, ok := v.(sentinel); ok {
				break
			}
			cols = append([]string{v.(Col).Name}, cols...)
		}
		m.push(doSelect(m.pop(), cols))
	})

	// --- sinks ---
	m.def("show", false, func(m *Machine) { doShow(m.pop()) })

	return m
}

func (m *Machine) binop(op string) {
	r := m.pop()
	l := m.pop()
	if isExpr(l) || isExpr(r) {
		m.push(Binop{Op: op, L: toExpr(l), R: toExpr(r)})
		return
	}
	li := int64(l.(VInt))
	ri := int64(r.(VInt))
	switch op {
	case "and":
		m.push(VInt(li & ri))
	case "or":
		m.push(VInt(li | ri))
	default:
		var f bool
		switch op {
		case ">":
			f = li > ri
		case "<":
			f = li < ri
		case "=":
			f = li == ri
		}
		if f {
			m.push(VInt(-1))
		} else {
			m.push(VInt(0))
		}
	}
}

func toExpr(v any) any {
	if isExpr(v) {
		return v
	}
	return Lit{V: v}
}

// ========================= where / select polymorphically =========================

func doWhere(rel, pred any) any {
	switch r := rel.(type) {
	case Query: // full version: CanPush? yes->stay lazy : force()->Tabular
		return Query{Node: Select{In: r.Node, Pred: pred}, Src: r.Src}
	case Tabular:
		var out [][]any
		for _, row := range r.Rows {
			if truthy(evalExpr(pred, r.Cols, row)) {
				out = append(out, row)
			}
		}
		return Tabular{Cols: r.Cols, Rows: out}
	}
	panic("where: not a relation on the stack")
}

func doSelect(rel any, cols []string) any {
	switch r := rel.(type) {
	case Query:
		return Query{Node: Project{In: r.Node, Cols: cols}, Src: r.Src}
	case Tabular:
		idx := make([]int, len(cols))
		for i, c := range cols {
			idx[i] = indexOf(r.Cols, c)
		}
		out := make([][]any, len(r.Rows))
		for i, row := range r.Rows {
			nr := make([]any, len(idx))
			for j, k := range idx {
				nr[j] = row[k]
			}
			out[i] = nr
		}
		return Tabular{Cols: cols, Rows: out}
	}
	panic("select: not a relation on the stack")
}

func doShow(rel any) {
	switch r := rel.(type) {
	case Query:
		if r.Src != nil && r.Src.DB != nil {
			printTable(force(r))
		} else {
			sql, args := compile(r)
			fmt.Println("[Query -> SQL]", sql, "  args:", args)
		}
	case Tabular:
		printTable(r)
	default:
		fmt.Println(valueStr(r))
	}
}

// force: compile a Query to SQL, run it against its Source's open DB, and
// materialize the result rows into a Tabular.
func force(q Query) Tabular {
	sqlText, args := compile(q)
	rows, err := q.Src.DB.Query(sqlText, args...)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		panic(err)
	}
	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			panic(err)
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	return Tabular{Cols: cols, Rows: out}
}

// ========================= Query -> SQL compiler =========================

type sqlParts struct {
	table  string
	cols   []string
	wheres []string
	args   []any
}

func fold(n any, p *sqlParts) {
	switch t := n.(type) {
	case Scan:
		p.table = t.Table
	case Select:
		fold(t.In, p)
		p.wheres = append(p.wheres, exprSQL(t.Pred, &p.args))
	case Project:
		fold(t.In, p)
		p.cols = t.Cols
	}
}

func compile(pl Query) (string, []any) {
	var p sqlParts
	fold(pl.Node, &p)
	cols := "*"
	if len(p.cols) > 0 {
		cols = strings.Join(p.cols, ", ")
	}
	sql := "SELECT " + cols + " FROM " + p.table
	if len(p.wheres) > 0 {
		sql += " WHERE " + strings.Join(p.wheres, " AND ")
	}
	return sql, p.args
}

func exprSQL(e any, args *[]any) string {
	switch t := e.(type) {
	case Col:
		return t.Name
	case Lit:
		*args = append(*args, scalarGo(t.V))
		return "?"
	case Binop:
		return "(" + exprSQL(t.L, args) + " " + sqlOp(t.Op) + " " + exprSQL(t.R, args) + ")"
	}
	return "?"
}

func sqlOp(op string) string {
	switch op {
	case "and":
		return "AND"
	case "or":
		return "OR"
	}
	return op
}

// ========================= Expr evaluator (Tabular path) =========================

func evalExpr(e any, cols []string, row []any) any {
	switch t := e.(type) {
	case Col:
		return row[indexOf(cols, t.Name)]
	case Lit:
		return t.V
	case Binop:
		return applyOp(t.Op, evalExpr(t.L, cols, row), evalExpr(t.R, cols, row))
	}
	return nil
}

func applyOp(op string, l, r any) any {
	switch op {
	case "and":
		return truthy(l) && truthy(r)
	case "or":
		return truthy(l) || truthy(r)
	}
	l, r = coerce(l, r)
	switch lv := l.(type) {
	case VInt:
		rv := r.(VInt)
		switch op {
		case ">":
			return lv > rv
		case "<":
			return lv < rv
		case "=":
			return lv == rv
		}
	case VStr:
		rv := r.(VStr)
		switch op {
		case ">":
			return lv > rv
		case "<":
			return lv < rv
		case "=":
			return lv == rv
		}
	}
	return false
}

// ========================= helpers =========================

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case VInt:
		return t != 0
	}
	return false
}
func toInt(v any) (int64, bool) {
	switch t := v.(type) {
	case VInt:
		return int64(t), true
	case VStr:
		n, err := strconv.ParseInt(string(t), 10, 64)
		return n, err == nil
	}
	return 0, false
}
func coerce(l, r any) (any, any) {
	if li, ok := toInt(l); ok {
		if ri, ok2 := toInt(r); ok2 {
			return VInt(li), VInt(ri)
		}
	}
	return VStr(toStr(l)), VStr(toStr(r))
}
func toStr(v any) string {
	switch t := v.(type) {
	case VStr:
		return string(t)
	case VInt:
		return strconv.FormatInt(int64(t), 10)
	}
	return fmt.Sprintf("%v", v)
}
func scalarGo(v any) any {
	switch t := v.(type) {
	case VInt:
		return int64(t)
	case VStr:
		return string(t)
	}
	return v
}
func indexOf(s []string, x string) int {
	for i, v := range s {
		if v == x {
			return i
		}
	}
	panic("unknown column: " + x)
}
func valueStr(v any) string {
	switch t := v.(type) {
	case VInt:
		return strconv.FormatInt(int64(t), 10)
	case VStr:
		return `"` + string(t) + `"`
	case Query:
		return "<plan>"
	case Tabular:
		return fmt.Sprintf("<tabular %dx%d>", len(t.Rows), len(t.Cols))
	case Col:
		return "col:" + t.Name
	case Binop:
		return "<expr>"
	case Lit:
		return "lit:" + valueStr(t.V)
	}
	return fmt.Sprintf("%v", v)
}

func loadCSV(fn string) Tabular {
	f, err := os.Open(fn)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		panic(err)
	}
	if len(recs) == 0 {
		return Tabular{}
	}
	var rows [][]any
	for _, rec := range recs[1:] {
		row := make([]any, len(rec))
		for i, cell := range rec {
			if n, err := strconv.ParseInt(cell, 10, 64); err == nil {
				row[i] = VInt(n)
			} else {
				row[i] = VStr(cell)
			}
		}
		rows = append(rows, row)
	}
	return Tabular{Cols: recs[0], Rows: rows}
}

func printTable(t Tabular) {
	fmt.Println(strings.Join(t.Cols, " | "))
	for _, row := range t.Rows {
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = toStr(c)
		}
		fmt.Println(strings.Join(cells, " | "))
	}
}

// ========================= lexer =========================

func tokenize(s string) []string {
	var toks []string
	i := 0
	isSpace := func(b byte) bool { return b == ' ' || b == '\n' || b == '\t' || b == '\r' }
	for i < len(s) {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		if (s[i] == 'S' || s[i] == 's') && i+1 < len(s) && s[i+1] == '"' { // S" string literal
			i += 2
			for i < len(s) && s[i] == ' ' {
				i++
			}
			start := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			toks = append(toks, "\x00"+strings.TrimRight(s[start:i], " "))
			if i < len(s) {
				i++
			}
			continue
		}
		start := i
		for i < len(s) && !isSpace(s[i]) {
			i++
		}
		toks = append(toks, s[start:i])
	}
	return toks
}

// ========================= REPL =========================

func (m *Machine) runLine(line string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(" ! error:", r)
			m.stack = m.stack[:0]
			m.state = Interpret
			m.cur = nil
		}
	}()
	m.toks = tokenize(line)
	m.pos = 0
	m.feed()
}

func repl(m *Machine) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		m.runLine(sc.Text())
		if m.state == Compile {
			fmt.Println("  compiled...")
		} else {
			fmt.Println(" ok")
		}
	}
}

func main() {
	_ = os.WriteFile("people.csv",
		[]byte("name,age,country\nAnna,34,PL\nBob,22,US\nCeline,41,PL\nDmitri,29,RU\n"), 0644)

	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open", dbFile, ":", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Fprintln(os.Stderr, "cannot connect to", dbFile, ":", err)
		os.Exit(1)
	}

	m := newMachine()
	m.src.DB = db
	if len(os.Args) > 1 { // file mode
		b, _ := os.ReadFile(os.Args[1])
		m.runLine(string(b))
		return
	}
	repl(m) // interactive mode (reads stdin)
}
