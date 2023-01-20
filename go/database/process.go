package vertabelo2sqlx

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

type TypeMapper interface {
	GoType(sqlType string, nullable bool) (out string)
}

type Processor struct {
	Package    string
	TypeMapper TypeMapper
}

func NewProcessor() *Processor {
	p := &Processor{}
	p.Package = "main"

	return p
}

func (p *Processor) convertTableName(t string) string {
	ary := strings.Split(t, "_")

	res := ""

	for _, a := range ary {
		res = res + strings.Title(a)
	}
	return res
}

func (p *Processor) convertColumnName(t string) string {
	ary := strings.Split(strings.ToLower(t), "_")

	res := ""

	for _, a := range ary {
		if a == "id" {
			res = res + "ID"
		} else {
			res = res + strings.Title(a)
		}
	}
	return res
}

func (p *Processor) convertType(c Column) string {

	if p.TypeMapper == nil {
		if c.Nullable == "true" {
			return "*string"
		} else {
			return "string"
		}
	}

	return p.TypeMapper.GoType(c.Type, c.Nullable == "true")

}

func (p *Processor) Process(in io.Reader, out io.Writer) error {

	xml, err := ioutil.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading xml failed: %s", err)
	}

	database, err := Parse(xml)
	if err != nil {
		return fmt.Errorf("parsing vertabelo xml failed: %s", err)
	}

	model := &SqlxDatabase{}
	model.Package = p.Package

	id2model := make(map[string]*Struct)
	id2field := make(map[string]*Field)

	for _, t := range database.Tables {

		s := &Struct{}

		s.Name = p.convertTableName(t.Name)
		s.SQLName = t.Name

		for _, c := range t.Columns {
			f := &Field{}
			f.Name = p.convertColumnName(c.Name)
			f.Type = p.convertType(c)
			f.Annotation = fmt.Sprintf("db:\"%s\"", c.Name)
			s.Fields = append(s.Fields, f)

			id2field[c.ID] = f
		}
		model.Structs = append(model.Structs, s)

		id2model[t.ID] = s
	}

	err = model.Emit(out)
	if err != nil {
		return fmt.Errorf("writing sqlx struct failed: %s", err)
	}

	return nil
}
