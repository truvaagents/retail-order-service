package main

import "testing"

func TestPriceRequestUsesMatchedSKU(t *testing.T) {
	cases := []struct {
		sku       string
		unitPrice int
	}{
		{"COFFEE-250", 1000},
		{"FILTER-100", 700},
		{"MUG-350", 2500},
	}
	for _, tc := range cases {
		product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-" + tc.sku, SKU: tc.sku, Quantity: 1})
		if err != nil {
			t.Fatalf("sku %s: unexpected error: %v", tc.sku, err)
		}
		if product.SKU != tc.sku {
			t.Fatalf("sku %s: expected selected SKU %s, got %s", tc.sku, tc.sku, product.SKU)
		}
		if product.UnitPrice != tc.unitPrice {
			t.Fatalf("sku %s: expected unit price %d, got %d", tc.sku, tc.unitPrice, product.UnitPrice)
		}
		if discount != 0 {
			t.Fatalf("sku %s: expected no discount, got %d", tc.sku, discount)
		}
	}
}
