package main

type Purchase struct {
	ID	int
	PurchaseNo	string
	ClientID	int
}

type Product struct {
	ID	int
	ProductCategoryID	int
	Sku	string
	Name	string
	Price	string
	Description	string
	Image	[]byte
}

type PurchaseItem struct {
	ID	int
	PurchaseID	int
	ProductID	int
	Amount	int
}

type ProductCategory struct {
	ID	int
	Name	string
	ParentCategoryID	*int
}

type Client struct {
	ID	int
	FullName	string
	Email	string
}

