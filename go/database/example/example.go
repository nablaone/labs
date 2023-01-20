package main

type Purchase struct {
	ID         int    `db:"id"`
	PurchaseNo string `db:"purchase_no"`
	ClientID   int    `db:"client_id"`
}

type Product struct {
	ID                int    `db:"id"`
	ProductCategoryID int    `db:"product_category_id"`
	Sku               string `db:"sku"`
	Name              string `db:"name"`
	Price             string `db:"price"`
	Description       string `db:"description"`
	Image             []byte `db:"image"`
}

type PurchaseItem struct {
	ID         int `db:"id"`
	PurchaseID int `db:"purchase_id"`
	ProductID  int `db:"product_id"`
	Amount     int `db:"amount"`
}

type ProductCategory struct {
	ID               int    `db:"id"`
	Name             string `db:"name"`
	ParentCategoryID *int   `db:"parent_category_id"`
}

type Client struct {
	ID       int    `db:"id"`
	FullName string `db:"full_name"`
	Email    string `db:"email"`
}
