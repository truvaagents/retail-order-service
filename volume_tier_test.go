package main

import "testing"

// Contract ORD-04: the 5% volume adjustment applies to quantities 10-24
// inclusive, so quantity exactly 10 must receive the adjustment.
func TestVolumeTierInclusiveMinimum(t *testing.T) {
	product, discount, err := priceRequest(CheckoutRequest{OrderID: "volume-tier-min-order", SKU: "COFFEE-250", Quantity: 10})
	if err != nil {
		t.Fatalf("priceRequest returned error: %v", err)
	}
	if product.UnitPrice != 1000 {
		t.Fatalf("unit price = %d, want 1000", product.UnitPrice)
	}
	if discount != 500 {
		t.Fatalf("discount_cents = %d, want 500", discount)
	}
	result := checkout(&Receipt{UnitPrice: product.UnitPrice, Quantity: 10, Discount: discount})
	if result.Total != 9500 {
		t.Fatalf("total = %d, want 9500", result.Total)
	}
	if result.Writes != 1 {
		t.Fatalf("ledger writes = %d, want 1", result.Writes)
	}
}
