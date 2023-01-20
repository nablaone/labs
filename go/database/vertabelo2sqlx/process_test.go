package vertabelo2sqlx_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/nablaone/vertabelo2sqlx"
)

func TestEmit1(t *testing.T) {

	in, err := os.Open("example/test.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	var p vertabelo2sqlx.Processor
	p.Package = "test"

	p.Process(in, ioutil.Discard)
	//p.Process(in, os.Stdout)

}
