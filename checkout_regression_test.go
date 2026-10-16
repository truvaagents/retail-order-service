package main

import "testing"

func TestCheckoutWelcomeDiscountAppliesOncePerOrder(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 3, Discount: 100})
	if got.Total != 2900 || got.Writes != 1 {
		t.Fatalf("WELCOME100 must subtract 100 cents once from the order subtotal: got %+v, want total 2900", got)
	}
}
