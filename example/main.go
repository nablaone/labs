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

func allClients(db *gorm.DB) {
	var clients []Client
	must(db.Find(&clients).GetErrors()...)

	for _, c := range clients {
		fmt.Println("* ", c.ID, c.FullName, c.Email)
		clientPurchase(db, &c)
		fmt.Println()
	}
}

func clientPurchase(db *gorm.DB, client *Client) {

	var purchases []Purchase

	must(db.Where("client_id = ?", client.ID).Find(&purchases).GetErrors()...)

	for _, p := range purchases {
		fmt.Println("  ", p.PurchaseNo)
	}
}

func main() {
	fmt.Println("Start")
	db, err := gorm.Open("postgres", "host=localhost port=5432 user=test dbname=test password=test")
	must(err)

	//db.LogMode(true)
	db.SingularTable(true)

	allClients(db)

	defer db.Close()
	fmt.Println("Done")
}
