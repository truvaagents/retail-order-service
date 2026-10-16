package main

import "testing"

func TestCheckoutWelcomePromotionAppliesOnce(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 3, Discount: 100})
	if got.Total != 2900 {
		t.Fatalf("WELCOME100 must subtract 100 cents once from the subtotal: got total %d, want 2900", got.Total)
	}
	if got.Writes != 1 {
		t.Fatalf("unexpected ledger writes: got %d, want 1", got.Writes)
	}
}
