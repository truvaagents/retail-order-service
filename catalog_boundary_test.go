package main

import "testing"

func TestPriceRequestAcceptsPublishedMaximumQuantity(t *testing.T) {
	const sku = "FILTER-100"
	max := 0
	for _, product := range catalog {
		if product.SKU == sku {
			max = product.MaxOrderQuantity
		}
	}
	if max == 0 {
		t.Fatalf("catalogue does not publish %s", sku)
	}
	product, _, err := priceRequest(CheckoutRequest{OrderID: "order-max", SKU: sku, Quantity: max})
	if err != nil {
		t.Fatalf("quantity equal to published maximum %d for %s was rejected: %v", max, sku, err)
	}
	if product.SKU != sku {
		t.Fatalf("expected %s, got %q", sku, product.SKU)
	}
}
