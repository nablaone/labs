package main

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

func must(errs ...error) {

	for _, e := range errs {
		if e != nil {
			panic(e)
		}
	}
}

func main() {
	fmt.Println("Start")
	db, err := gorm.Open("postgres", "host=localhost port=5432 user=test dbname=test password=test")
	must(err)

	db.LogMode(true)
	db.SingularTable(true)

	var clients []Client
	must(db.Find(&clients).GetErrors()...)

	for _, c := range clients {
		fmt.Println(c.ID, c.FullName, c.Email)
	}

	defer db.Close()
	fmt.Println("Done")
}
