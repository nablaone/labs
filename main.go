package main

import (
	"log"
	"os"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	pkgName := os.Args[1]
	file := os.Args[2]
	outFile := os.Args[3]

	f, err := os.Open(file)
	defer f.Close()
	checkErr(err)
	p, err := ParsePackage(pkgName, f)
	checkErr(err)

	out, err := os.Create(outFile)
	defer out.Close()
	checkErr(err)
	err = p.Render(out)
	checkErr(err)
}
