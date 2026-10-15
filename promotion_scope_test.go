package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWelcome100PersistedOrderSubtotal(t *testing.T) {
	s, _ := testService()
	request := CheckoutRequest{OrderID: "welcome-subtotal-order", SKU: "COFFEE-250", Quantity: 2, PromotionCode: "WELCOME100"}
	created, receipt := submit(t, s, request)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, receipt = %+v", created.Code, receipt)
	}
	read := httptest.NewRecorder()
	s.routes().ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/orders/"+request.OrderID, nil))
	if read.Code != http.StatusOK {
		t.Fatalf("stored order read status = %d", read.Code)
	}
	var order Order
	if err := json.Unmarshal(read.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}
	if order.Request != request || order.Receipt.UnitPrice != 1000 || order.Receipt.Discount != 100 || order.Receipt.Currency != "USD" {
		t.Fatalf("stored request or pricing changed: %+v", order)
	}
	// The pinned README applies 100 cents once to the 2000-cent subtotal.
	if order.Ledger.Total != 1900 {
		t.Fatalf("persisted order total = %d cents; want 1900 cents", order.Ledger.Total)
	}
	if order.Ledger.Writes != 1 {
		t.Fatalf("persisted ledger writes = %d; want 1", order.Ledger.Writes)
	}
	if receipt.LedgerTotal == nil || *receipt.LedgerTotal != order.Ledger.Total || receipt.LedgerWrites == nil || *receipt.LedgerWrites != 1 {
		t.Fatalf("response disagrees with stored order: %+v", receipt)
	}
}
