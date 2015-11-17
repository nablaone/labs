package main

import (
	"fmt"
	"github.com/nablaone/govertabelo"
	"io/ioutil"
	"log"
	"os"
)

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err)
		os.Exit(1)
	}
}

func describe_command(args []string) {

	if len(args) != 1 {
		log.Fatalln("missing file name")
	}

	filename := args[0]

	input, err := ioutil.ReadFile(filename)
	checkErr(err)

	m, err := govertabelo.Parse(input)
	checkErr(err)

	for _, t := range m.Tables {
		fmt.Println(t.Name)
		for _, c := range t.Columns {
			fmt.Printf("  %-20s %-20s\n", c.Name, c.Type)
		}
	}
}

func useApi(fn func(string, string) ([]byte, error), args []string) {

	if len(args) != 2 && len(args) != 3 {
		log.Fatalln("Wrong number of parameters")
	}

	var modelId string
	var versionId string
	var filename string

	if len(args) == 2 {
		modelId = args[0]
		versionId = ""
		filename = args[1]
	} else if len(args) == 3 {
		modelId = args[0]
		versionId = args[1]
		filename = args[2]
	}

	err := govertabelo.InitApi()
	checkErr(err)

	content, err := fn(modelId, versionId)
	checkErr(err)

	err = ioutil.WriteFile(filename, content, 0600)
	checkErr(err)
}

func getXml(args []string) {

	var fn = func(modelId, versionId string) (content []byte, err error) {
		return govertabelo.GetXML(modelId, versionId)
	}
	useApi(fn, args)
}

func getSql(args []string) {

	var fn = func(modelId, versionId string) (content []byte, err error) {
		return govertabelo.GetSQL(modelId, versionId)
	}
	useApi(fn, args)

}

func usage_command() {

	fmt.Println(`
vertabelo: command line tool for Vertabelo.com

commands:
describe filename.xml 
xml  model_GID filename.xml
sql  model_GID filename.sql

`)

}

func main() {

	args := os.Args[1:]

	if len(args) == 0 {
		usage_command()
		return
	}

	command := args[0]
	args = args[1:]

	if command == "describe" {
		describe_command(args)
	} else if command == "xml" {
		getXml(args)
	} else if command == "sql" {
		getSql(args)
	} else {
		log.Fatalf("Unknown command %s\n", command)
	}

}
