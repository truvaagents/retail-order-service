package main

import (
	"context"
	"net/http"
	"testing"
)

func TestWelcome100OncePerOrderPersistedLedger(t *testing.T) {
	s, m := testService()
	request := CheckoutRequest{OrderID: "welcome-once-five", SKU: "COFFEE-250", Quantity: 5, PromotionCode: "WELCOME100"}
	response, receipt := submit(t, s, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status: got %d want 201", response.Code)
	}
	stored, err := m.ReadOrder(context.Background(), request.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	// Pinned README: 1000 cents * 5 items - 100 cents once = 4900 cents.
	if stored.Ledger.Total != 4900 || stored.Ledger.Writes != 1 {
		t.Fatalf("persisted once-per-order ledger: got %+v want total=4900 writes=1", stored.Ledger)
	}
	if stored.Request != request || stored.Receipt.UnitPrice != 1000 || stored.Receipt.Discount != 100 || stored.Receipt.Currency != "USD" {
		t.Fatalf("accepted request or pricing snapshot changed: %+v", stored)
	}
	if receipt.LedgerTotal == nil || *receipt.LedgerTotal != 4900 || receipt.LedgerWrites == nil || *receipt.LedgerWrites != 1 {
		t.Fatalf("create receipt disagrees with persisted ledger: %+v", receipt)
	}
	response, replay := submit(t, s, request)
	if response.Code != http.StatusOK || response.Header().Get("Idempotency-Replayed") != "true" || replay.LedgerTotal == nil || *replay.LedgerTotal != 4900 || replay.LedgerWrites == nil || *replay.LedgerWrites != 1 {
		t.Fatalf("retry changed accepted outcome: status=%d receipt=%+v", response.Code, replay)
	}
	after, err := m.ReadOrder(context.Background(), request.OrderID)
	if err != nil || len(m.orders) != 1 || after.Ledger != stored.Ledger || after.Request != stored.Request || after.RequestSHA256 != stored.RequestSHA256 {
		t.Fatalf("retry mutated stored order: err=%v order=%+v", err, after)
	}
}
