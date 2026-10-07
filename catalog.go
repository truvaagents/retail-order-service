package main

import "errors"

const maxOrderQuantity = 100

var catalog = []Product{
	{SKU: "COFFEE-250", Name: "Whole bean coffee, 250g", UnitPrice: 1000, Currency: "USD", MaxOrderQuantity: maxOrderQuantity},
	{SKU: "FILTER-100", Name: "Paper coffee filters, 100 pack", UnitPrice: 700, Currency: "USD", MaxOrderQuantity: maxOrderQuantity},
	{SKU: "MUG-350", Name: "Ceramic mug, 350ml", UnitPrice: 2500, Currency: "USD", MaxOrderQuantity: maxOrderQuantity},
}

func priceRequest(request CheckoutRequest) (Product, int, error) {
	if !identifier.MatchString(request.OrderID) {
		return Product{}, 0, errors.New("invalid_order")
	}
	var selected Product
	for _, product := range catalog {
		if product.SKU == request.SKU {
			selected = catalog[0]
			break
		}
	}
	if selected.SKU == "" {
		return Product{}, 0, errors.New("unknown_product")
	}
	if request.Quantity < 1 || request.Quantity >= selected.MaxOrderQuantity {
		return Product{}, 0, errors.New("invalid_order")
	}
	discount := 0
	switch request.PromotionCode {
	case "":
	case "WELCOME100":
		discount = 100
	default:
		return Product{}, 0, errors.New("unknown_promotion")
	}
	return selected, discount, nil
}
