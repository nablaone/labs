package vertabelo2gorm

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

type Processor struct {
	Package string
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

func (p *Processor) convertType(t string) string {

	switch strings.ToLower(t) {

	case "int4":
		return "int"

	default:
		return "string"
	}
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

	model := &GormDatabase{}
	model.Package = p.Package

	for _, t := range database.Tables {
		var m Model
		m.Name = p.convertTableName(t.Name)
		m.SQLName = t.Name

		for _, c := range t.Columns {
			var f Field
			f.Name = p.convertColumnName(c.Name)
			f.Type = p.convertType(c.Type)
			m.Fields = append(m.Fields, f)
		}
		model.Models = append(model.Models, m)
	}

	err = model.Emit(out)
	if err != nil {
		return fmt.Errorf("writing GORM model failed: %s", err)
	}

	return nil
}
