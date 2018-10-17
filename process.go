package vertabelo2gorm

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

type Processor struct {
	Package     string
	TypeMapping map[string]string
}

func NewProcessor() *Processor {
	p := &Processor{}
	p.Package = "main"

	p.TypeMapping = make(map[string]string)
	// FIXME only types that exists in test.xml
	p.TypeMapping["int"] = "int"
	p.TypeMapping["varchar(255)"] = "string"
	p.TypeMapping["varchar(1000)"] = "string"
	p.TypeMapping["char(12)"] = "string"
	p.TypeMapping["bytea"] = "[]byte"
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

	res := ""

	if c.Nullable == "true" {
		res = "*"
	}

	sql := strings.ToLower(c.Type)

	gotype, ok := p.TypeMapping[sql]

	if ok {
		res = res + gotype
	} else {
		res = res + "string"
	}

	return res

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

	id2model := make(map[string]*Model)
	id2field := make(map[string]*Field)

	for _, t := range database.Tables {

		m := &Model{}

		m.Name = p.convertTableName(t.Name)
		m.SQLName = t.Name

		for _, c := range t.Columns {
			f := &Field{}
			f.Name = p.convertColumnName(c.Name)
			f.Type = p.convertType(c)
			m.Fields = append(m.Fields, f)

			id2field[c.ID] = f
		}
		model.Models = append(model.Models, m)

		id2model[t.ID] = m
	}

	for _, r := range database.References {
		primaryModel := id2model[r.PKTable]

		if primaryModel == nil {
			panic("No such model: " + r.PKTable)
		}

		foreignModel := id2model[r.FKTable]

		if len(r.ReferenceColumns) > 2 {
			panic("Multi column referecenes are not supported. Reference id: " + r.Name)
		}

		foreignField := id2field[r.ReferenceColumns[0].FKColumn]

		pkField := &Field{}
		pkField.Name = foreignModel.Name + "s"
		pkField.Type = "[]" + foreignModel.Name
		pkField.Annotation = "gorm:foreignkey:" + foreignField.Name

		primaryModel.Fields = append(primaryModel.Fields, pkField)

		fkField := &Field{}
		fkField.Name = primaryModel.Name
		fkField.Type = "*" + primaryModel.Name
		fkField.Annotation = "gorm:foreignkey:" + foreignField.Name

		foreignModel.Fields = append(foreignModel.Fields, fkField)

	}

	err = model.Emit(out)
	if err != nil {
		return fmt.Errorf("writing GORM model failed: %s", err)
	}

	return nil
}
