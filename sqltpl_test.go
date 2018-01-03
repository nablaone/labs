package sqltpl_test

import (
	"strings"
	"testing"

	"github.com/nablaone/sqltpl"
)

func assertParam(t *testing.T, p sqltpl.Param, goName, goType, name string) {

	if p.GoName != goName {
		t.Fatalf("invalid parameter goname. got %s, expected %s", p.GoName, goName)

	}

	if p.GoType != goType {
		t.Fatalf("invalid parameter gotype. got %s, expected %s", p.GoType, goType)
	}

	if p.Name != name {
		t.Fatalf("invalid parameter name. got %s, expected %s", p.Name, name)
	}

}

func TestSimpleQuery(t *testing.T) {

	parser := sqltpl.NewSQLParser()

	query := `
-- sqltpl: Test1 

select a@@int from foo wher id = ?id@@int

-- end
	
	`

	bundle, err := parser.Parse(strings.NewReader(query))

	if err != nil {
		t.Error(err)
	}

	if len(bundle.Queries) != 1 {
		t.Fatal("should be 1 query")
	}

	q := bundle.Queries[0]

	if len(q.Ins) != 1 {
		t.Fatal("must be 1 input parameter")
	}

	in := q.Ins[0]

	assertParam(t, in, "Id", "int", "id")

	if len(q.Outs) != 1 {
		t.Fatal("must be 1 output parameter")
	}

	out := q.Outs[0]

	assertParam(t, out, "A", "int", "a")

}

func TestParams(t *testing.T) {

	parser := sqltpl.NewSQLParser()

	query := `
-- sqltpl: Test1 

select a@@int,
	   b@@string,
	   c@@sql.NullString 

from foo where 
	    id1 = ?id1@@int
	and id2 = ?id2@@string
	and id3 = ?id3@@sql.NullString

-- end
	
	`

	bundle, err := parser.Parse(strings.NewReader(query))

	if err != nil {
		t.Error(err)
	}

	if len(bundle.Queries) != 1 {
		t.Fatal("must be 1 query")
	}

	q := bundle.Queries[0]

	if len(q.Ins) != 3 {
		t.Fatalf("must be 3 input parameter but got %d", len(q.Ins))
	}
	assertParam(t, q.Ins[0], "Id1", "int", "id1")
	assertParam(t, q.Ins[1], "Id2", "string", "id2")
	assertParam(t, q.Ins[2], "Id3", "sql.NullString", "id3")

	if len(q.Outs) != 3 {
		t.Fatalf("must be 3 output parameter but got %d", len(q.Ins))
	}
	assertParam(t, q.Outs[0], "A", "int", "a")
	assertParam(t, q.Outs[1], "B", "string", "b")
	assertParam(t, q.Outs[2], "C", "sql.NullString", "c")
}
