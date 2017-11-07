package sqltpl

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"
)

const (
	InsertQuery = iota
	SelectQuery
	ScalarSelectQuery
	UpdateQuery
	DeleteQuery
)

type Param struct {
	Name   string
	GoType string
	GoName string
}

type Query struct {
	Name  string
	Ins   []Param
	Outs  []Param
	Query string
}

type QueryBundle struct {
	Name    string
	Queries []*Query
}

func processInParams(query string) (string, []Param, error) {
	rx := regexp.MustCompile("\\?[_a-zA-Z]+@@[a-z]+")

	var ins []Param
	var count = 0
	var repl = func(in string) string {
		count++
		in = in[1:]
		ary := strings.Split(in, "@@")

		name := ary[0]

		p := Param{
			Name:   name,
			GoType: ary[1],
			GoName: strings.Title(name),
		}

		ins = append(ins, p)
		return fmt.Sprintf("$%d", count) // FIXME must be parametrized
	}

	resQuery := rx.ReplaceAllStringFunc(query, repl)
	return resQuery, ins, nil

}

func processOutParams(query string) (string, []Param, error) {
	rx := regexp.MustCompile("[_a-zA-Z]+@@[a-z]+")

	var ins []Param

	var repl = func(in string) string {
		ary := strings.Split(in, "@@")

		p := Param{
			Name:   ary[0],
			GoType: ary[1],
			GoName: strings.Title(ary[0]),
		}

		ins = append(ins, p)
		return p.Name
	}

	resQuery := rx.ReplaceAllStringFunc(query, repl)
	return resQuery, ins, nil

}

func parseQuery(query string) (Query, error) {

	q, ins, err := processInParams(query)

	q, outs, err := processOutParams(q)

	t := Query{
		Name:  "",
		Ins:   ins,
		Outs:  outs,
		Query: q,
	}
	return t, err
}

func (t *Query) process() error {

	q, ins, err := processInParams(t.Query)
	if err != nil {
		return err
	}

	q, outs, err := processOutParams(q)
	if err != nil {
		return err
	}

	t.Ins = ins
	t.Outs = outs
	t.Query = q

	return nil
}

const (
	beginToken   = "-- sqltpl:"
	endToken     = "-- end"
	startComment = "--"
)

type Parser struct {
	TransformLine func(string) string
	Context       string
}

func NewSqlParser() *Parser {
	return &Parser{
		TransformLine: func(s string) string {
			return s
		},
	}
}

func NewGoParser() *Parser {
	var comment = "//"

	return &Parser{
		TransformLine: func(s string) string {
			line := strings.TrimLeft(s, "\t ")

			if strings.HasPrefix(line, comment) {

				line = line[len(comment):]
				line = strings.TrimLeft(line, "\t ")

				return line
			}

			return ""
		},
	}
}

func (p *Parser) Parse(r io.Reader) (*QueryBundle, error) {
	var res QueryBundle
	scanner := bufio.NewScanner(r)

	var lineNumber int
	var current *Query

	for scanner.Scan() {
		line := p.TransformLine(scanner.Text())
		lineNumber++
		switch {
		case strings.HasPrefix(line, beginToken):

			name := strings.TrimSpace(line[len(beginToken):])
			current = &Query{}
			current.Name = name

		case strings.HasPrefix(line, endToken):

			if current == nil {
				return nil, fmt.Errorf("unexpected end token: %s:%d", p.Context, lineNumber)
			}

			res.Queries = append(res.Queries, current)
			current = nil

		case strings.HasPrefix(line, beginToken):
			// eat comments
		default:

			if current != nil {
				current.Query = current.Query + line
			}
		}
	}

	for _, q := range res.Queries {
		err := q.process()
		if err != nil {
			return nil, err
		}

	}

	return &res, nil
}

func (p *QueryBundle) Render(w io.Writer) error {

	err := p.renderHelper(w)
	if err != nil {
		return nil
	}

	tmpl, err := template.New("test").Parse(gocode)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, p)

	if err != nil {
		return err
	}

	return nil
}

func (p *QueryBundle) renderHelper(w io.Writer) error {
	tmpl, err := template.New("helper").Parse(helper)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, p)

	if err != nil {
		return err
	}

	return nil

}

const gocode = `

{{range .Queries}}
type {{.Name}}Query struct {
{{range .Ins}}
	{{.GoName}} {{.GoType}}{{end}}
}

type {{.Name}}Row struct {
{{range .Outs}}
	{{.GoName}} {{.GoType}}{{end}}
}

func (s *sqlTplQ) {{.Name}}(in {{.Name}}Query) ([]{{.Name}}Row, error) {

	var res []{{.Name}}Row

	rows, err := s.q.Query("{{.Query}}", {{range $i, $v := .Ins}}{{if $i}}, {{end}}in.{{$v.GoName}}{{end}})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var out {{.Name}}Row

		if err := rows.Scan({{range $i, $v := .Outs}}{{if $i}}, {{end}}out.{{$v.GoName}}{{end}}); err != nil {
			return nil, err
		}
		res = append(res, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

{{end}}
`

const helper = `
package {{.Name}}

// generate by go-sqltpl do not edit 

import (
	"database/sql"
)

type sqlTplQuerer interface {

	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type sqlTplQ struct {
	q sqlTplQuerer
}

func WithDB(db *sql.DB) *sqlTplQ {
	return &sqlTplQ{
		q: db,
	}
}

func WithTX(tx *sql.Tx) *sqlTplQ {
	return &sqlTplQ{
		q: tx,
	}
}


`
