package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/nablaone/sqltpl"
)

var output = flag.String("o", "queries.sqltpl.go", "output file name")
var packageName = flag.String("p", "main", "package name")

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func parse(file string) (*sqltpl.QueryBundle, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	var parser *sqltpl.Parser

	switch {

	case strings.HasSuffix(file, ".go"):
		parser = sqltpl.NewGoParser()

	case strings.HasSuffix(file, ".sql") || strings.HasSuffix(file, ".sqlt"):
		parser = sqltpl.NewSQLParser()

	default:
		return nil, errors.New("file type not supported")
	}

	parser.Context = file
	qp, err := parser.Parse(f)

	if err != nil {
		return nil, err
	}
	return qp, nil
}

func main() {

	flag.Parse()

	var bundle sqltpl.QueryBundle
	bundle.Name = *packageName

	for _, file := range flag.Args() {
		qb, err := parse(file)
		checkErr(err)

		bundle.Queries = append(bundle.Queries, qb.Queries...)
	}

	out, err := os.Create(*output)
	defer out.Close()
	checkErr(err)

	err = bundle.Render(out)
	checkErr(err)

}
