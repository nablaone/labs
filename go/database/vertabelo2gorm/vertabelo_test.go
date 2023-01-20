package vertabelo2gorm_test

import (
	"io/ioutil"
	"testing"

	"github.com/nablaone/vertabelo2gorm"
)

func TestParse(t *testing.T) {

	b, err := ioutil.ReadFile("example/test.xml")

	if err != nil {
		t.Fatal(err)

	}

	_, err = vertabelo2gorm.Parse(b)

	if err != nil {
		t.Fatal(err)
	}

}

func TestPurchaseTable(t *testing.T) {

	b, err := ioutil.ReadFile("example/test.xml")

	if err != nil {
		t.Fatal(err)

	}

	db, err := vertabelo2gorm.Parse(b)

	if err != nil {
		t.Fatal(err)
	}

	if len(db.Tables) != 5 {
		t.Error("Expected 6 tables but got ", len(db.Tables))
		t.Fail()
	}

	table := db.Tables[0]

	if table.Name != "purchase" {
		t.Error("first table must be purchase")
		t.Fail()
	}

	if len(table.Columns) != 3 {
		t.Fail()
	}

	if len(table.PrimaryKey.ColumnID) != 1 {
		t.Fail()
	}

	if table.PrimaryKey.ColumnID[0] != "c1" {
		t.Fail()
	}

}
