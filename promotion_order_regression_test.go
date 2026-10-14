package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWelcome100PersistedOrderSubtotal(t *testing.T) {
	// README.md at owner-pinned 9b8dae6dde77512050847af998410764716655cd:
	// WELCOME100 subtracts 100 cents once from the order subtotal.
	for _, tc := range []struct {
		name string
		quantity int
		promotion string
		want int
	}{
		{"two_items_promotion", 2, "WELCOME100", 1900},
		{"one_item_promotion", 1, "WELCOME100", 900},
		{"two_items_no_promotion", 2, "", 2000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := testService()
			request := CheckoutRequest{OrderID: "promotion-" + tc.name, SKU: "COFFEE-250", Quantity: tc.quantity, PromotionCode: tc.promotion}
			response, receipt := submit(t, s, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("create status: got %d want 201", response.Code)
			}
			stored, err := m.ReadOrder(context.Background(), request.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Ledger.Total != tc.want {
				t.Fatalf("persisted order total: got %d cents want %d cents", stored.Ledger.Total, tc.want)
			}
			if stored.Request != request || stored.Receipt.UnitPrice != 1000 || stored.Receipt.Currency != "USD" || stored.Ledger.Writes != 1 || len(m.orders) != 1 {
				t.Fatalf("stored request/pricing/write invariant: %+v", stored)
			}
			if receipt.LedgerTotal == nil || *receipt.LedgerTotal != tc.want || receipt.LedgerWrites == nil || *receipt.LedgerWrites != 1 {
				t.Fatalf("create receipt effect: %+v", receipt)
			}
			read := httptest.NewRecorder()
			s.routes().ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/orders/"+request.OrderID, nil))
			var retrieved Order
			if err := json.Unmarshal(read.Body.Bytes(), &retrieved); err != nil {
				t.Fatal(err)
			}
			if read.Code != http.StatusOK || retrieved.Ledger.Total != tc.want || retrieved.Ledger.Writes != 1 {
				t.Fatalf("order read effect: status=%d order=%+v", read.Code, retrieved)
			}
			response, replay := submit(t, s, request)
			if response.Code != http.StatusOK || response.Header().Get("Idempotency-Replayed") != "true" || replay.LedgerTotal == nil || *replay.LedgerTotal != tc.want || replay.LedgerWrites == nil || *replay.LedgerWrites != 1 || len(m.orders) != 1 || m.orders[request.OrderID].Ledger != stored.Ledger {
				t.Fatalf("retry changed persisted effect: status=%d receipt=%+v", response.Code, replay)
			}
		})
	}
}
