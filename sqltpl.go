package sqltpl

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
)

// Param holds query parameter info.
type Param struct {
	Name   string
	GoType string
	GoName string
}

// Query represents SQL query.
type Query struct {
	Name  string
	Ins   []Param
	Outs  []Param
	Query string
}

// QueryBundle contains all currently processed queries.
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

func (q *Query) process() error {

	query, ins, err := processInParams(q.Query)
	if err != nil {
		return err
	}

	query, outs, err := processOutParams(query)
	if err != nil {
		return err
	}

	q.Ins = ins
	q.Outs = outs
	q.Query = query

	return nil
}

const (
	beginToken   = "-- sqltpl:"
	endToken     = "-- end"
	startComment = "--"
)

// Parser contains current parsing context.
type Parser struct {
	TransformLine func(string) string
	Context       string
}

// NewSQLParser creates new parser for  *.sqlt files.
func NewSQLParser() *Parser {
	return &Parser{
		TransformLine: func(s string) string {
			return s
		},
	}
}

// NewGoParser create new parser for *.go type files.
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

// Parse parses queries found in reader.
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
				current.Query = current.Query + " " + line
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

func (q *Query) render(tpl string, w io.Writer) error {

	tmpl, err := template.New("test").Parse(tpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, q)

	if err != nil {
		return err
	}

	return nil
}

// Render renders Go code.
func (p *QueryBundle) Render(w io.Writer) error {

	err := p.renderHelper(w)
	if err != nil {
		return nil
	}

	for _, q := range p.Queries {

		tpl := q.findTemplate()

		err := q.render(tpl, w)

		if err != nil {
			return err
		}
	}

	return nil
}

func (q *Query) findTemplate() string {
	if len(q.Outs) == 0 {
		return executeTemplate
	}
	return selectTemplate
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

const executeTemplate = `

func (s *SqlTplQ) {{.Name}}({{range $i, $v := .Ins}}{{if $i}}, {{end}}{{$v.GoName}} {{$v.GoType}}{{end}}) ( error) {
	
	_, err := s.q.Exec("{{.Query}}", {{range $i, $v := .Ins}}{{if $i}}, {{end}}{{$v.GoName}} {{end}})
	if err != nil {
		return err
	}
	return nil	
}	

`

const selectTemplate = `

type {{.Name}}Query struct {
{{range .Ins}}
	{{.GoName}} {{.GoType}}{{end}}
}

type {{.Name}}Row struct {
{{range .Outs}}
	{{.GoName}} {{.GoType}}{{end}}
}

func (s *SqlTplQ) {{.Name}}(in {{.Name}}Query) ([]{{.Name}}Row, error) {

	var res []{{.Name}}Row

	rows, err := s.q.Query("{{.Query}}", {{range $i, $v := .Ins}}{{if $i}}, {{end}}in.{{$v.GoName}}{{end}})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var out {{.Name}}Row

		if err := rows.Scan({{range $i, $v := .Outs}}{{if $i}}, {{end}}&out.{{$v.GoName}}{{end}}); err != nil {
			return nil, err
		}
		res = append(res, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

`

const helper = `
package {{.Name}}

// generate by go-sqltpl do not edit 

import (
	"database/sql"
)

// Mothods supported by transaction and bare connection
type SqlTplQuerer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// Wraps underlying querer
type SqlTplQ struct {
	q SqlTplQuerer
}

// Runs queries against database connection
func WithDB(db *sql.DB) *SqlTplQ {
	return &SqlTplQ{
		q: db,
	}
}

// Runs queries against database transaction
func WithTX(tx *sql.Tx) *SqlTplQ {
	return &SqlTplQ{
		q: tx,
	}
}


`
