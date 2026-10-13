package main

import (
	"context"
	"net/http"
	"testing"
)

func TestWelcome100OrderSubtotalPersisted(t *testing.T) {
	for _, tc := range []struct {
		name string
		quantity int
		promotion string
		discount, total int
	}{
		{"five_promoted", 5, "WELCOME100", 100, 4900},
		{"one_promoted", 1, "WELCOME100", 100, 900},
		{"five_unpromoted", 5, "", 0, 5000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := testService()
			request := CheckoutRequest{OrderID: "promotion-" + tc.name, SKU: "COFFEE-250", Quantity: tc.quantity, PromotionCode: tc.promotion}
			response, first := submit(t, s, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("acceptance status: got %d want 201", response.Code)
			}
			if first.UnitPrice != 1000 || first.Discount != tc.discount || first.Currency != "USD" {
				t.Fatalf("intermediate pricing: %+v", first)
			}
			stored, err := m.ReadOrder(context.Background(), request.OrderID)
			if err != nil { t.Fatal(err) }
			if stored.Request != request || stored.Receipt.UnitPrice != 1000 || stored.Receipt.Discount != tc.discount {
				t.Fatalf("stored request/pricing: %+v", stored)
			}
			if stored.Ledger.Total != tc.total || stored.Ledger.Writes != 1 {
				t.Fatalf("persisted ledger: got total=%d writes=%d want total=%d writes=1", stored.Ledger.Total, stored.Ledger.Writes, tc.total)
			}
			if first.LedgerTotal == nil || *first.LedgerTotal != tc.total || first.LedgerWrites == nil || *first.LedgerWrites != 1 {
				t.Fatalf("accepted ledger receipt: %+v", first)
			}
			s.build.Revision = "build-next"
			response, replay := submit(t, s, request)
			if response.Code != http.StatusOK || response.Header().Get("Idempotency-Replayed") != "true" || replay.ObservationID == first.ObservationID || replay.Revision != "build-next" || replay.OrderRevision != first.OrderRevision {
				t.Fatalf("replay semantics: status=%d receipt=%+v", response.Code, replay)
			}
			if replay.UnitPrice != 1000 || replay.Discount != tc.discount || replay.LedgerTotal == nil || *replay.LedgerTotal != tc.total || replay.LedgerWrites == nil || *replay.LedgerWrites != 1 {
				t.Fatalf("replayed pricing/ledger: %+v", replay)
			}
			after, err := m.ReadOrder(context.Background(), request.OrderID)
			if err != nil { t.Fatal(err) }
			if after.Request != stored.Request || after.RequestSHA256 != stored.RequestSHA256 || after.Ledger != stored.Ledger || after.Receipt.Revision != stored.Receipt.Revision || len(m.orders) != 1 {
				t.Fatalf("replay changed immutable order: %+v", after)
			}
		})
	}
}
