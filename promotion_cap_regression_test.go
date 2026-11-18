package main

import (
	"net/http"
	"testing"
)

func TestPromotionCapScope(t *testing.T) {
	// ORD-04 through ORD-06: 11000 - (550 + min(1045,1000)) = 9450.
	t.Run("observed_persisted_total", func(t *testing.T) {
		s, m := testService()
		request := CheckoutRequest{OrderID: "cap-scope-observed", SKU: "COFFEE-250", Quantity: 11, PromotionCode: "SAVE10"}
		response, receipt := submit(t, s, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("acceptance status=%d, want=201", response.Code)
		}
		stored, ok := m.orders[request.OrderID]
		if !ok {
			t.Fatal("accepted order missing")
		}
		if stored.Ledger.Total != 9450 {
			t.Errorf("persisted total=%d, want=9450", stored.Ledger.Total)
		}
		if receipt.Discount != 1550 || stored.Receipt.Discount != 1550 {
			t.Errorf("combined discounts receipt=%d stored=%d, want=1550", receipt.Discount, stored.Receipt.Discount)
		}
		if receipt.LedgerTotal == nil {
			t.Error("response ledger total missing")
		} else if *receipt.LedgerTotal != 9450 {
			t.Errorf("response ledger total=%d, want=9450", *receipt.LedgerTotal)
		}
		if stored.Ledger.Writes != 1 || len(m.orders) != 1 {
			t.Errorf("logical writes=%d records=%d, want=1 each", stored.Ledger.Writes, len(m.orders))
		}
	})

	// Synthetic unit fixtures isolate cap boundaries without changing the catalogue.
	// Exact-cent adjustments avoid unrelated rounding and tier-boundary defects.
	for _, tc := range []struct {
		name, sku, code string
		price, quantity, discount, total int
	}{
		{"below_cap", "COFFEE-test", "SAVE10", 9990, 1, 999, 8991},
		{"at_cap", "COFFEE-test", "SAVE10", 10000, 1, 1000, 9000},
		{"above_cap", "COFFEE-test", "SAVE10", 10010, 1, 1000, 9010},
		{"volume_promotion_below_cap", "COFFEE-test", "SAVE10", 500, 20, 1450, 8550},
		{"volume_promotion_above_cap", "COFFEE-test", "SAVE10", 1000, 11, 1550, 9450},
		{"higher_volume_above_cap", "COFFEE-test", "SAVE10", 1000, 30, 4000, 26000},
		{"no_promotion", "COFFEE-test", "", 1000, 11, 550, 10450},
		{"fixed_promotion", "COFFEE-test", "WELCOME100", 1000, 11, 650, 10350},
		// 20000 subtotal, 1000 volume, 2850 promotion: combined 3850.
		{"uncapped_percentage", "MUG-test", "MUG15", 1000, 20, 3850, 16150},
		{"subtotal_clamp", "COFFEE-test", "WELCOME100", 50, 1, 50, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			discount, err := orderDiscount(Product{SKU: tc.sku, UnitPrice: tc.price}, tc.quantity, tc.code)
			if err != nil {
				t.Fatal(err)
			}
			ledger := checkout(&Receipt{UnitPrice: tc.price, Quantity: tc.quantity, Discount: discount})
			if discount != tc.discount || ledger.Total != tc.total || ledger.Writes != 1 {
				t.Fatalf("discount=%d total=%d writes=%d, want discount=%d total=%d writes=1", discount, ledger.Total, ledger.Writes, tc.discount, tc.total)
			}
		})
	}
}
