package main

import "testing"

func TestCheckout(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		price, quantity, discount, total int
	}{
		{"one_item", 1000, 1, 0, 1000}, {"multiple_items", 700, 3, 0, 2100}, {"welcome_promotion", 1000, 1, 100, 900},
		{"order_adjustment", 1000, 3, 100, 2900},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkout(&Receipt{UnitPrice: tc.price, Quantity: tc.quantity, Discount: tc.discount})
			if got.Total != tc.total || got.Writes != 1 {
				t.Fatalf("unexpected order: %+v", got)
			}
		})
	}
}

func TestCataloguePricing(t *testing.T) {
	for _, item := range catalog {
		product, discount, err := priceRequest(CheckoutRequest{OrderID: "catalog-order", SKU: item.SKU, Quantity: 1})
		if err != nil || product != item || discount != 0 {
			t.Fatalf("catalogue price: %+v, discount=%d, error=%v", product, discount, err)
		}
	}
}

func TestOrderPricing(t *testing.T) {
	for _, tc := range []struct {
		name, sku, code string
		quantity, total int
	}{
		{"volume", "COFFEE-250", "", 12, 11400},
		{"large_order", "COFFEE-250", "", 30, 27000},
		{"welcome", "COFFEE-250", "WELCOME100", 3, 2900},
		{"volume_and_welcome", "FILTER-100", "WELCOME100", 12, 7880},
		{"percentage", "TEA-100", "SAVE10", 8, 6300},
		{"promotion_limit", "MUG-350", "SAVE10", 6, 14000},
		{"product_promotion", "MUG-350", "MUG15", 2, 4250},
	} {
		t.Run(tc.name, func(t *testing.T) {
			product, discount, err := priceRequest(CheckoutRequest{OrderID: "priced-order", SKU: tc.sku, Quantity: tc.quantity, PromotionCode: tc.code})
			if err != nil {
				t.Fatal(err)
			}
			result := checkout(&Receipt{UnitPrice: product.UnitPrice, Quantity: tc.quantity, Discount: discount})
			if result.Total != tc.total {
				t.Fatalf("total=%d, want=%d", result.Total, tc.total)
			}
		})
	}
}

func TestPromotionEligibility(t *testing.T) {
	for _, request := range []CheckoutRequest{
		{OrderID: "small-order", SKU: "COFFEE-250", Quantity: 1, PromotionCode: "SAVE10"},
		{OrderID: "coffee-order", SKU: "COFFEE-250", Quantity: 2, PromotionCode: "MUG15"},
	} {
		if _, _, err := priceRequest(request); err == nil || err.Error() != "promotion_not_eligible" {
			t.Fatalf("promotion eligibility: %v", err)
		}
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
