package main

import "testing"

func TestWelcomePromotionSubtotalScope(t *testing.T) {
	s, _ := testService()
	response, receipt := submit(t, s, CheckoutRequest{OrderID: "welcome-subtotal", SKU: "COFFEE-250", Quantity: 9, PromotionCode: "WELCOME100"})
	if response.Code != 201 {
		t.Fatalf("create: %d %+v", response.Code, receipt)
	}
	if receipt.LedgerTotal == nil || *receipt.LedgerTotal != 8900 || receipt.LedgerWrites == nil || *receipt.LedgerWrites != 1 {
		t.Fatalf("WELCOME100 must subtract 100 once from the subtotal: %+v", receipt)
	}
}
