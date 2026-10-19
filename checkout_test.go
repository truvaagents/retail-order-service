package main

import "testing"

func TestCheckout(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		price, quantity, discount, total int
	}{
		{"one_item", 1000, 1, 0, 1000}, {"multiple_items", 700, 3, 0, 2100}, {"welcome_promotion", 1000, 1, 100, 900},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkout(&Receipt{UnitPrice: tc.price, Quantity: tc.quantity, Discount: tc.discount})
			if got.Total != tc.total || got.Writes != 1 {
				t.Fatalf("unexpected order: %+v", got)
			}
		})
	}
}
func TestPriceRequest(t *testing.T) {
	product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-1", SKU: "COFFEE-250", Quantity: 2, PromotionCode: "WELCOME100"})
	if err != nil || product.UnitPrice != 1000 || product.MaxOrderQuantity != 100 || discount != 100 {
		t.Fatalf("unexpected catalogue result: %+v %d %v", product, discount, err)
	}
	if _, _, err := priceRequest(CheckoutRequest{OrderID: "order-2", SKU: "FILTER-100", Quantity: 99}); err != nil {
		t.Fatalf("valid bulk order rejected: %v", err)
	}
	for _, request := range []CheckoutRequest{
		{OrderID: "../x", SKU: "COFFEE-250", Quantity: 1}, {OrderID: "order-1", SKU: "missing", Quantity: 1},
		{OrderID: "order-1", SKU: "COFFEE-250", Quantity: 0}, {OrderID: "order-1", SKU: "COFFEE-250", Quantity: 101},
		{OrderID: "order-1", SKU: "COFFEE-250", Quantity: -1},
		{OrderID: "order-1", SKU: "COFFEE-250", Quantity: 1, PromotionCode: "missing"},
	} {
		if _, _, err := priceRequest(request); err == nil {
			t.Fatalf("accepted invalid request: %+v", request)
		}
	}
}
func TestCheckoutPromotionOncePerOrder(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 2, Discount: 100})
	if got.Total != 1900 || got.Writes != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}
