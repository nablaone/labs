package main

//go:generate sqltpl main sample.sqlt sample_sqlt.go

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

const (
	DB_USER     = "test"
	DB_PASSWORD = "test"
	DB_NAME     = "test"
)

func main() {
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
		DB_USER, DB_PASSWORD, DB_NAME)
	db, err := sql.Open("postgres", dbinfo)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Start")

	outs, _ := UserIdNameByEmail(db, UserIdNameByEmailIn{Email: "foo@bar"})
	log.Println(outs)
}
