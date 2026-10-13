package main

import (
	"context"
	"net/http"
	"testing"
)

func TestWelcome100OnceOnPersistedOrderSubtotal(t *testing.T) {
	s, store := testService()
	request := CheckoutRequest{OrderID: "promotion-scope-regression", SKU: "COFFEE-250", Quantity: 5, PromotionCode: "WELCOME100"}
	response, _ := submit(t, s, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d", response.Code, http.StatusCreated)
	}
	order, err := store.ReadOrder(context.Background(), request.OrderID)
	if err != nil {
		t.Fatalf("read persisted order: %v", err)
	}
	if order.Request != request || order.Receipt.UnitPrice != 1000 || order.Receipt.Discount != 100 || order.Receipt.Currency != "USD" {
		t.Fatalf("unexpected immutable request or pricing: %+v", order)
	}
	// The pinned contract subtracts 100 cents once from the 5000-cent subtotal.
	if order.Ledger.Total != 4900 {
		t.Fatalf("persisted ledger total: got %d cents, want 4900 cents", order.Ledger.Total)
	}
	if order.Ledger.Writes != 1 || len(store.orders) != 1 {
		t.Fatalf("persisted ledger writes: got %+v across %d orders, want one write", order.Ledger, len(store.orders))
	}
}
