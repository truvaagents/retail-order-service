package main

import "testing"

func TestCheckoutWelcomePromotionQuantityTwo(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 2, Discount: 100})
	if got.Total != 1900 || got.Writes != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}
