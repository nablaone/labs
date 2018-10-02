package example

type Purchase struct {
	ID	string
	PurchaseNo	string
	ClientID	string
}

type Product struct {
	ID	string
	ProductCategoryID	string
	Sku	string
	Name	string
	Price	string
	Description	string
	Image	string
}

type PurchaseItem struct {
	ID	string
	PurchaseID	string
	ProductID	string
	Amount	string
}

type ProductCategory struct {
	ID	string
	Name	string
	ParentCategoryID	string
}

type Client struct {
	ID	string
	FullName	string
	Email	string
}

