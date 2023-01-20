package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func must(errs ...error) {

	for _, e := range errs {
		if e != nil {
			panic(e)
		}
	}
}

func allClients(db *sqlx.DB) {
	var clients []Client

	err := db.Select(&clients, "SELECT * FROM client")
	must(err)
	for _, c := range clients {
		fmt.Println("* ", c.ID, c.FullName, c.Email)
		clientPurchase(db, &c)
		fmt.Println()
	}
}

func clientPurchase(db *sqlx.DB, client *Client) {

	var purchases []Purchase

	err := db.Select(&purchases, "SELECT * FROM purchase WHERE client_id = $1", client.ID)
	must(err)

	for _, p := range purchases {
		fmt.Println("  ", p.PurchaseNo)

		var items []PurchaseItem

		err = db.Select(&items, "SELECT * FROM purchase_item where purchase_id = $1 ", p.ID)
		must(err)
		//must(db.Model(&p).Preload("Product").Related(&items).GetErrors()...)

		for _, i := range items {
			var product Product
			err = db.Get(&product, "SELECT * FROM product where id = $1", i.ProductID)
			must(err)
			fmt.Println("   - ", product.Sku, i.Amount)
		}
	}

}

func main() {
	fmt.Println("Start")
	db, err := sqlx.Open("postgres", "host=localhost port=5432 user=test dbname=test password=test sslmode=disable")

	must(err)

	allClients(db)

	defer db.Close()
	fmt.Println("Done")
}
