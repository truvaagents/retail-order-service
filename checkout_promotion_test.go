package main

import "testing"

func TestCheckoutPromotionSubtractsOnce(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 3, Discount: 100})
	if got.Total != 2900 {
		t.Fatalf("promoted multi-unit order total = %d, want 2900", got.Total)
	}
	if got.Writes != 1 {
		t.Fatalf("ledger writes = %d, want 1", got.Writes)
	}
}
