package main

//go:generate go-sqltpl  sample.sqlt main.go

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

const (
	dbUser = "test"
	dbPass = "test"
	dbName = "test"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
		dbUser, dbPass, dbName)
	db, err := sql.Open("postgres", dbinfo)

	checkErr(err)

	log.Println("Start")

	q := WithDB(db)

	// -- sqltpl: DropDb
	// drop table foo;
	// -- end
	_ = q.DropDb()

	// -- sqltpl: InitDb
	// create table foo (bar int null);
	// -- end
	err = q.InitDb()
	checkErr(err)

	// -- sqltpl: AddFoo
	// insert into foo values(?n@@int)
	// -- end

	x := []int{1, 2, 3, 4, 5, 6}

	for _, v := range x {
		checkErr(q.AddFoo(v))
	}

	// -- sqltpl: Content
	// select bar@@int from foo
	// -- end

	rows, err := q.Content()
	checkErr(err)
	log.Println("rows", rows)

	txExample(db)

	nullableExample(db)
}

func txExample(db *sql.DB) error {
	// -- sqltpl: GetByID
	// select bar@@int from foo where bar = ?id@@int
	// -- end

	log.Println("with tx")

	tx, err := db.Begin()
	checkErr(err)

	q := WithTX(tx)

	q.AddFoo(100)

	rows, err := q.GetByID(100)
	log.Println("should be 1: ", len(rows))

	err = tx.Rollback()
	checkErr(err)

	rows, err = WithDB(db).GetByID(100)
	log.Println("should be 0: ", len(rows))
	return nil
}

func intN(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{
			Int64: 0,
			Valid: false}
	}

	return sql.NullInt64{
		Int64: int64(*i),
		Valid: true}
}

func nullableExample(db *sql.DB) error {

	tx, err := db.Begin()
	checkErr(err)

	q := WithTX(tx)

	// -- sqltpl: AddFooN
	//  insert into foo(bar) values(?bar@@sql.NullInt64)
	// -- end

	var i *int

	q.AddFooN(intN(i))

	j := -1

	q.AddFooN(intN(&j))

	// -- sqltpl: ContentNullable
	// select bar@@sql.NullInt64 from foo
	// -- end

	rows, err := q.ContentNullable()
	checkErr(err)

	fmt.Println(rows)

	err = tx.Rollback()
	checkErr(err)

	return nil
}
