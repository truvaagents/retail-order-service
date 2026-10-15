package main

import "testing"

// TestCheckoutPromotedMultiUnitLedgerTotal asserts the ledger total persisted for
// a promoted multi-unit order. The pinned contract (README.md at revision
// 9b8dae6dde77512050847af998410764716655cd) states WELCOME100 subtracts 100 cents
// once from the order subtotal regardless of item count, so a 7-item COFFEE-250
// order at 1000 cents with WELCOME100 must persist 1000*7-100 = 6900 cents.
func TestCheckoutPromotedMultiUnitLedgerTotal(t *testing.T) {
	got := checkout(&Receipt{UnitPrice: 1000, Quantity: 7, Discount: 100})
	if got.Total != 6900 {
		t.Fatalf("promoted multi-unit ledger total = %d, want 6900", got.Total)
	}
	if got.Writes != 1 {
		t.Fatalf("promoted multi-unit ledger writes = %d, want 1", got.Writes)
	}
}
