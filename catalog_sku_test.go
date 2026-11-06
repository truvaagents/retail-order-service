package main

import "testing"

func TestPriceRequestSelectsMatchedSKU(t *testing.T) {
	product, discount, err := priceRequest(CheckoutRequest{OrderID: "order-mug-1", SKU: "MUG-350", Quantity: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.SKU != "MUG-350" || product.UnitPrice != 2500 || discount != 0 {
		t.Fatalf("MUG-350 must be priced from its own catalogue entry: %+v discount=%d", product, discount)
	}
	if total := checkout(&Receipt{UnitPrice: product.UnitPrice, Quantity: 1, Discount: discount}); total.Total != 2500 || total.Writes != 1 {
		t.Fatalf("unexpected order total for MUG-350 quantity 1: %+v", total)
	}
	filter, filterDiscount, err := priceRequest(CheckoutRequest{OrderID: "order-filter-1", SKU: "FILTER-100", Quantity: 3})
	if err != nil || filter.SKU != "FILTER-100" || filter.UnitPrice != 700 || filterDiscount != 0 {
		t.Fatalf("FILTER-100 must be priced from its own catalogue entry: %+v discount=%d err=%v", filter, filterDiscount, err)
	}
}
