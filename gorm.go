package vertabelo2gorm

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
)

type Field struct {
	Name       string
	Type       string
	Annotation string
}

type Model struct {
	Name    string
	SQLName string
	Fields  []Field
}

// FIXME add other nodess 	Annotation string
type GormDatabase struct {
	Package string
	Imports []string
	Models  []Model
}

func (g *GormDatabase) Emit(w io.Writer) error {

	var buff bytes.Buffer

	fmt.Fprintf(&buff, "package %s\n\n", g.Package)

	for _, i := range g.Imports {
		fmt.Fprintf(&buff, "import \"%s\"\n", i)
	}

	for _, m := range g.Models {

		fmt.Fprintf(&buff, "type %s struct {\n", m.Name)

		for _, f := range m.Fields {

			if f.Annotation == "" {
				fmt.Fprintf(&buff, "\t%s\t%s\n", f.Name, f.Type)
			} else {
				fmt.Fprintf(&buff, "\t%s\t%s\t`%s`\n", f.Name, f.Type, f.Annotation)
			}

		}

		fmt.Fprintf(&buff, "}\n\n")
	}

	src := buff.String()
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return fmt.Errorf("generated code parsing failed: %s", err)
	}

	err = format.Node(w, fset, f)
	if err != nil {
		return fmt.Errorf("generated code formating failed: %s", err)
	}

	return nil
}
