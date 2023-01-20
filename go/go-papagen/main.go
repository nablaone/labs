package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func process(source string, outPkg string, typeReplace map[string]string) (string, error) {

	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "", source, 0)
	if err != nil {
		return "", err
	}

	//ast.Print(fset, f)

	f.Name.Name = outPkg

	// Inspect the AST and print all identifiers and literals.
	ast.Inspect(f, func(n ast.Node) bool {

		switch x := n.(type) {
		case *ast.TypeSpec:

			var t, ok = typeReplace[x.Name.Name]

			if x.Assign != 0 && ok {

				//((*ast.Ident)x.Type).Name = t

				v := x.Type.(*ast.Ident)
				v.Name = t

				/*
					switch y := x.Type.(type) {
					case *ast.Ident:
						y.Name = t
					}
				*/
			}
		case *ast.Package:
			fmt.Println(x)
		}
		return true
	})

	var b bytes.Buffer

	err = printer.Fprint(&b, fset, f)
	if err != nil {
		return "", nil
	}

	return string(b.Bytes()), nil
}

func parseTypeReplacements(types []string) map[string]string {
	res := make(map[string]string)

	for _, t := range types {
		ary := strings.Split(t, "=")
		if len(ary) != 2 {
			panic("invalid type mapping: " + t)

		}
		res[ary[0]] = ary[1]
	}

	return res
}

func main() {

	var must = func(err error) {
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	}

	if len(os.Args) < 4 {
		fmt.Println("Usage:")
		fmt.Println(os.Args[0], ": input_file.go destination_package T=foo [ T1=bar ] ")
		os.Exit(2)
	}

	srcFile := os.Args[1]
	outPkg := os.Args[2]
	outFile := outPkg + "/" + path.Base(srcFile)

	m := parseTypeReplacements(os.Args[3:])

	b, err := ioutil.ReadFile(srcFile)
	must(err)

	src := string(b)

	out, err := process(src, outPkg, m)
	must(err)

	os.MkdirAll(outPkg, 0777)
	err = ioutil.WriteFile(outFile, []byte(out), 0666)
	must(err)

}
