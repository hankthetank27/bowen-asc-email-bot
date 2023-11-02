package controllers

type Email struct {
	Sender     string
	Recipients []string
	Subject    string
	Body       string
}

type EmailTemplate struct {
	Locals   *NewOrderLocals
	Purchase Purchase
}

type NewOrderLocals struct {
	OrderNumber  string
	OrderId      string
	CustomerInfo struct {
		FirstName string
		LastName  string
		Email     string
		Phone     string
	}
	Purchases []Purchase
}

type Purchase struct {
	ProductSKU     string
	PurchaseType   string
	SubjectAddress string
}

type SqSpaceOrders struct {
	Orders []SqSpaceOrder `json:"result"`
}

type SqSpaceOrder struct {
	Id             string `json:"id"`
	CustomerEmail  string `json:"customerEmail"`
	OrderNumber    string `json:"orderNumber"`
	BillingAddress struct {
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		Address1    string `json:"address1"`
		Address2    string `json:"address2"`
		City        string `json:"city"`
		State       string `json:"state"`
		CountryCode string `json:"countryCode"`
		PostalCode  string `json:"postalCode"`
		Phone       string `json:"phone"`
	} `json:"billingAddress"`
	LineItems []struct {
		ProductSKU     string `json:"sku"`
		ProductName    string `json:"productName"`
		Customizations *[]struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"customizations"`
	} `json:"LineItems"`
}
