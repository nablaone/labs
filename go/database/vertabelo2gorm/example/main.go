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
	must(db.Preload("Purchases").Find(&clients).GetErrors()...)

	for _, c := range clients {
		fmt.Println("* ", c.ID, c.FullName, c.Email)
		clientPurchase(db, &c)
		fmt.Println()
	}
}

func clientPurchase(db *gorm.DB, client *Client) {

	for _, p := range client.Purchases {
		fmt.Println("  ", p.PurchaseNo)

		var items []PurchaseItem
		must(db.Model(&p).Preload("Product").Related(&items).GetErrors()...)

		for _, i := range items {
			fmt.Println("   - ", i.Product.Sku, i.Amount)
		}
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
