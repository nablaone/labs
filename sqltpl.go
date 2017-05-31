package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"
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

type Package struct {
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
		return fmt.Sprintf("$%d", count)
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
		Name:  "foo",
		Ins:   ins,
		Outs:  outs,
		Query: q,
	}
	return t, err
}

func (t *Query) process() {

	q, ins, _ := processInParams(t.Query)

	q, outs, _ := processOutParams(q)

	t.Ins = ins
	t.Outs = outs
	t.Query = q

}

func ParsePackage(name string, r io.Reader) (Package, error) {
	var res Package
	res.Name = name
	scanner := bufio.NewScanner(r)

	var t *Query

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "-- Query: "):

			name := line[len("-- Query: "):]
			t = &Query{}

			res.Queries = append(res.Queries, t)
			t.Name = name

		case strings.HasPrefix(line, "--"):

		default:

			if t != nil {
				t.Query = t.Query + line
			}

		}

	}

	for _, t := range res.Queries {
		t.process()
	}

	return res, nil
}

func (p *Package) Render(w io.Writer) error {

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

const gocode = `
package {{.Name}}

import (
	"database/sql"
)

type Querer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

{{range .Queries}}
type {{.Name}}In struct {
{{range .Ins}}
	{{.GoName}} {{.GoType}}{{end}}
}

type {{.Name}}Out struct {
{{range .Outs}}
	{{.GoName}} {{.GoType}}{{end}}
}


func {{.Name}}(q Querer, in {{.Name}}In) ([]{{.Name}}Out, error) {

	var res []{{.Name}}Out

	rows, err := q.Query("{{.Query}}", {{range $i, $v := .Ins}}{{if $i}}, {{end}}in.{{$v.GoName}}{{end}})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var out {{.Name}}Out

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
