package factory

import (
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func NewPurchase() *models.Purchase {
	return &models.Purchase{
		ShipTo:       "Peter",
		Address:      "123 Spooner St.",
		City:         "Quahog",
		State:        "RI",
		PostalCode:   "90210",
		CountryCode:  "US",
		AddressNotes: "asdf",
		Items: []models.PurchaseItem{
			{
				Quantity: "1",
				Options: []models.PurchaseItemOption{
					{
						Name:  "size",
						Value: "large",
					},
					{
						Name:  "color",
						Value: "red",
					},
				},
				Shipping: models.PurchaseShippingOption{
					Name:    "Worldwide",
					Service: "Standard",
				},
				Memo: "I want it fast!",
			},
		},
		AlternateContactInfo: "peter@protonmail.com",
		PricingCoin:          "MCK",
	}
}
