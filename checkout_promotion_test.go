package main

import "testing"

// TestCheckoutWelcomePromotionOncePerOrder pins the owner-approved contract:
// WELCOME100 subtracts 100 cents once from the order subtotal, regardless of
// the number of items in the order.
func TestCheckoutWelcomePromotionOncePerOrder(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 3, Discount: 100})
	if got.Total != 2900 || got.Writes != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}
