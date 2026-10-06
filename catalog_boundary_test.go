package main

import "testing"

func TestPriceRequestAcceptsMaxOrderQuantity(t *testing.T) {
	product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-max", SKU: "FILTER-100", Quantity: 100})
	if err != nil {
		t.Fatalf("quantity equal to MaxOrderQuantity should be accepted, got error: %v", err)
	}
	if product.SKU != "FILTER-100" || product.MaxOrderQuantity != 100 || discount != 0 {
		t.Fatalf("unexpected pricing result: product=%+v discount=%d", product, discount)
	}
}
