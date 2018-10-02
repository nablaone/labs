package vertabelo2gorm

import (
	"fmt"
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

	fmt.Fprintf(w, "package %s\n\n", g.Package)

	for _, i := range g.Imports {
		fmt.Fprintf(w, "import \"%s\"\n", i)
	}

	for _, m := range g.Models {

		fmt.Fprintf(w, "type %s struct {\n", m.Name)

		for _, f := range m.Fields {

			if f.Annotation == "" {
				fmt.Fprintf(w, "\t%s\t%s\n", f.Name, f.Type)
			} else {
				fmt.Fprintf(w, "\t%s\t%s\t`%s`\n", f.Name, f.Type, f.Annotation)
			}

		}

		fmt.Fprintf(w, "}\n\n")

	}

	return nil
}
