package main

import "testing"

func TestPriceRequestUsesMatchedSKUPrice(t *testing.T) {
	product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-mug", SKU: "MUG-350", Quantity: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.SKU != "MUG-350" || product.UnitPrice != 2500 || discount != 0 {
		t.Fatalf("matched SKU priced from wrong catalogue entry: %+v %d", product, discount)
	}
}
