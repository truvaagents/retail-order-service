package main

import "testing"

func TestPriceRequestUsesMatchedCatalogueSKU(t *testing.T) {
	for _, tc := range []struct {
		sku   string
		price int
	}{
		{"MUG-350", 2500},
		{"FILTER-100", 700},
		{"COFFEE-250", 1000},
	} {
		product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-1", SKU: tc.sku, Quantity: 1})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.sku, err)
		}
		if product.SKU != tc.sku {
			t.Fatalf("%s: priced from catalogue entry %q", tc.sku, product.SKU)
		}
		if product.UnitPrice != tc.price {
			t.Fatalf("%s: unit price %d, want %d", tc.sku, product.UnitPrice, tc.price)
		}
		if discount != 0 {
			t.Fatalf("%s: discount %d, want 0", tc.sku, discount)
		}
	}
}
