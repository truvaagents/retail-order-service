package main

import "testing"

func TestCheckoutAppliesOrderPromotionOnce(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 3, Discount: 100})
	if got.Total != 2900 || got.Writes != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}
