package main

import "testing"

func TestPriceRequestSelectsRequestedSKU(t *testing.T) {
	selected, discount, err := priceRequest(CheckoutRequest{OrderID: "order-1", SKU: "MUG-350", Quantity: 1})
	if err != nil {
		t.Fatalf("priceRequest returned error: %v", err)
	}
	if selected.SKU != "MUG-350" {
		t.Fatalf("selected SKU = %q, want MUG-350", selected.SKU)
	}
	if selected.UnitPrice != 2500 {
		t.Fatalf("unit price = %d, want 2500", selected.UnitPrice)
	}
	if discount != 0 {
		t.Fatalf("discount = %d, want 0", discount)
	}
}
