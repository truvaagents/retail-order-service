package main

import (
	"net/http"
	"testing"
)

func TestMatchedSKUSelectionAndLedger(t *testing.T) {
	// Expected cents are catalogue inputs; the pinned README requires
	// server catalogue pricing. Quantity one and no promotion isolate selection.
	for _, tc := range []struct {
		sku string
		price int
	}{
		{"COFFEE-250", 1000},
		{"FILTER-100", 700},
		{"MUG-350", 2500},
	} {
		t.Run(tc.sku, func(t *testing.T) {
			request := CheckoutRequest{OrderID: "selection-" + tc.sku, SKU: tc.sku, Quantity: 1}
			product, discount, err := priceRequest(request)
			if err != nil {
				t.Fatalf("valid catalogue request rejected: %v", err)
			}
			if product.SKU != tc.sku || product.UnitPrice != tc.price || product.Currency != "USD" || discount != 0 {
				t.Errorf("matched product identity and price: got %+v discount=%d; want SKU=%s USD cents=%d discount=0", product, discount, tc.sku, tc.price)
			}
			s, m := testService()
			response, receipt := submit(t, s, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("create status: got %d want 201", response.Code)
			}
			if receipt.SKU != tc.sku || receipt.UnitPrice != tc.price || receipt.Currency != "USD" || receipt.Discount != 0 {
				t.Errorf("accepted pricing: got %+v; want SKU=%s USD cents=%d discount=0", receipt, tc.sku, tc.price)
			}
			if receipt.LedgerTotal == nil || *receipt.LedgerTotal != tc.price || receipt.LedgerWrites == nil || *receipt.LedgerWrites != 1 {
				t.Errorf("response final ledger: got %+v; want total=%d writes=1", receipt, tc.price)
			}
			order, ok := m.orders[request.OrderID]
			if !ok || len(m.orders) != 1 || order.Request != request || order.Receipt.UnitPrice != tc.price || order.Receipt.Currency != "USD" || order.Ledger.Total != tc.price || order.Ledger.Writes != 1 {
				t.Errorf("persisted final ledger and pricing: got %+v present=%v orders=%d; want total=%d writes=1", order, ok, len(m.orders), tc.price)
			}
		})
	}
	t.Run("unknown_SKU", func(t *testing.T) {
		request := CheckoutRequest{OrderID: "selection-unknown", SKU: "UNKNOWN", Quantity: 1}
		product, _, err := priceRequest(request)
		if err == nil || err.Error() != "unknown_product" || product.SKU != "" {
			t.Fatalf("unknown SKU selection: product=%+v error=%v", product, err)
		}
		s, m := testService()
		response, receipt := submit(t, s, request)
		if response.Code != http.StatusUnprocessableEntity || receipt.ErrorCode != "unknown_product" || len(m.orders) != 0 {
			t.Fatalf("unknown SKU must not commit: status=%d receipt=%+v orders=%d", response.Code, receipt, len(m.orders))
		}
	})
}
