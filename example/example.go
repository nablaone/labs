package main

type Purchase struct {
	ID            int
	PurchaseNo    string
	ClientID      int
	PurchaseItems *PurchaseItem `gorm:foreignkey:PurchaseID`
	Client        *Client       `gorm:foreignkey:ClientID`
}

type Product struct {
	ID                int
	ProductCategoryID int
	Sku               string
	Name              string
	Price             string
	Description       string
	Image             []byte
	PurchaseItems     *PurchaseItem    `gorm:foreignkey:ProductID`
	ProductCategory   *ProductCategory `gorm:foreignkey:ProductCategoryID`
}

type PurchaseItem struct {
	ID         int
	PurchaseID int
	ProductID  int
	Amount     int
	Purchase   *Purchase `gorm:foreignkey:PurchaseID`
	Product    *Product  `gorm:foreignkey:ProductID`
}

type ProductCategory struct {
	ID               int
	Name             string
	ParentCategoryID *int
	ProductCategorys *ProductCategory `gorm:foreignkey:ParentCategoryID`
	ProductCategory  *ProductCategory `gorm:foreignkey:ParentCategoryID`
	Products         *Product         `gorm:foreignkey:ProductCategoryID`
}

type Client struct {
	ID        int
	FullName  string
	Email     string
	Purchases *Purchase `gorm:foreignkey:ClientID`
}
