package vertabelo2gorm_test

import (
	"os"
	"testing"

	"github.com/nablaone/vertabelo2gorm"
)

func TestEmit1(t *testing.T) {

	in, err := os.Open("example/test.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	var p vertabelo2gorm.Processor
	p.Package = "test"

	p.Process(in, os.Stdout)

}
