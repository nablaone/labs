package main

import (
	"fmt"
	"os"

	"github.com/nablaone/vertabelo2sqlx"
)

func main() {

	if len(os.Args) != 4 {
		fmt.Println("Usage: vertabelo2sqlx input.xml package_name output.go")
		return
	}

	in := os.Args[1]
	packageName := os.Args[2]
	out := os.Args[3]

	processor := vertabelo2sqlx.NewProcessor()
	processor.Package = packageName
	processor.TypeMapper = &vertabelo2sqlx.PostgreSQLTypeMapper{}

	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	fin, err := os.Open(in)
	must(err)

	fout, err := os.Create(out)
	must(err)

	err = processor.Process(fin, fout)
	must(err)
}
