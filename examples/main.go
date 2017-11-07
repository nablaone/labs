package main

//go:generate go-sqltpl  sample.sqlt main.go

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

	q := WithDB(db)

	outs, _ := q.UserIdNameByEmail(UserIdNameByEmailQuery{Email: "foo@bar"})
	log.Println("outs", outs)

	// -- sqltpl: Ids
	// select id@@int from user
	// -- end

	outs2, _ := q.Ids(IdsQuery{})
	log.Println("outs2", outs2)

}
