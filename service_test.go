package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/truvaagents/truva-g3/core"
)

type memoryStore struct {
	orders                    map[string]Order
	receipts                  map[string]Receipt
	commitErr, observationErr error
}

func (m *memoryStore) Ready(context.Context) error { return nil }
func (m *memoryStore) CreateOrder(_ context.Context, order Order) (bool, error) {
	if _, exists := m.orders[order.Request.OrderID]; exists {
		return false, nil
	}
	m.orders[order.Request.OrderID] = order
	return true, m.commitErr
}
func (m *memoryStore) ReadOrder(_ context.Context, id string) (Order, error) {
	order, ok := m.orders[id]
	if !ok {
		return order, errNotFound
	}
	return order, nil
}
func (m *memoryStore) SaveReceipt(_ context.Context, receipt Receipt) error {
	m.receipts[receipt.ObservationID] = receipt
	return m.observationErr
}
func (m *memoryStore) ReadReceipt(_ context.Context, id string) (Receipt, error) {
	r, ok := m.receipts[id]
	if !ok {
		return r, errNotFound
	}
	return r, nil
}
func (m *memoryStore) Index(context.Context, Receipt) error                { return nil }
func (m *memoryStore) ReadFeed(context.Context, Window, int) (Feed, error) { return Feed{}, nil }

func testService() (*service, *memoryStore) {
	m := &memoryStore{orders: map[string]Order{}, receipts: map[string]Receipt{}}
	return &service{store: m, token: strings.Repeat("x", 32), build: BuildInfo{Revision: "build-current"}, logger: core.NewProductionLogger(core.LoggingConfig{Level: "error", Format: "json"}, core.DevelopmentConfig{}, serviceName)}, m
}
func submit(t *testing.T, s *service, body CheckoutRequest) (*httptest.ResponseRecorder, Receipt) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+s.token)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, req)
	var receipt Receipt
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	return w, receipt
}

func TestOrderRetryAndConflict(t *testing.T) {
	s, m := testService()
	body := CheckoutRequest{OrderID: "order-1", SKU: "COFFEE-250", Quantity: 1}
	w, first := submit(t, s, body)
	if w.Code != 201 || first.OrderRevision != "build-current" {
		t.Fatalf("create: %d %+v", w.Code, first)
	}
	s.build.Revision = "build-next"
	w, replay := submit(t, s, body)
	if w.Code != 200 || w.Header().Get("Idempotency-Replayed") != "true" || replay.ObservationID == first.ObservationID || replay.Revision != "build-next" || replay.OrderRevision != "build-current" {
		t.Fatalf("replay: %d %+v", w.Code, replay)
	}
	body.Quantity = 2
	w, rejected := submit(t, s, body)
	if w.Code != 409 || rejected.ErrorCode != "order_id_conflict" || len(m.orders) != 1 || m.orders[body.OrderID].Request.Quantity != 1 {
		t.Fatalf("conflict changed order: %d %+v", w.Code, rejected)
	}
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/evidence/"+rejected.ObservationID, nil))
	var evidence Receipt
	_ = json.Unmarshal(w.Body.Bytes(), &evidence)
	if w.Code != 200 || evidence.LedgerTotal != nil || evidence.OrderRevision != "" {
		t.Fatalf("conflict inherited accepted outcome: %+v", evidence)
	}
}

func TestAttemptPromotionObservation(t *testing.T) {
	for _, promotion := range []string{"", "WELCOME100"} {
		t.Run("promotion_"+promotion, func(t *testing.T) {
			s, m := testService()
			request := CheckoutRequest{OrderID: "observed-order", SKU: "COFFEE-250", Quantity: 1, PromotionCode: promotion}
			for _, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
				response, receipt := submit(t, s, request)
				if response.Code != wantStatus || receipt.RequestedPromotionCode == nil || *receipt.RequestedPromotionCode != promotion {
					t.Fatalf("attempt capture: status=%d receipt=%+v", response.Code, receipt)
				}
				stored := m.receipts[receipt.ObservationID]
				if stored.RequestedPromotionCode == nil || *stored.RequestedPromotionCode != promotion {
					t.Fatal("attempt promotion missing from stored observation")
				}
			}
			request.Quantity = 2
			request.PromotionCode = "different-promotion"
			response, rejected := submit(t, s, request)
			if response.Code != http.StatusConflict || rejected.RequestedPromotionCode == nil || *rejected.RequestedPromotionCode != request.PromotionCode {
				t.Fatalf("rejected attempt capture: status=%d receipt=%+v", response.Code, rejected)
			}
			if m.orders[request.OrderID].Request.PromotionCode != promotion {
				t.Fatal("attempt capture changed immutable accepted request")
			}
			read := httptest.NewRecorder()
			s.routes().ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/evidence/"+rejected.ObservationID, nil))
			var evidence Receipt
			if err := json.Unmarshal(read.Body.Bytes(), &evidence); err != nil {
				t.Fatal(err)
			}
			if evidence.RequestedPromotionCode == nil || *evidence.RequestedPromotionCode != request.PromotionCode || evidence.LedgerTotal != nil {
				t.Fatalf("rejected evidence: %+v", evidence)
			}
		})
	}
}

func TestUnavailableAttemptPromotion(t *testing.T) {
	s, _ := testService()
	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"promotion_code":`))
	request.Header.Set("Authorization", "Bearer "+s.token)
	response := httptest.NewRecorder()
	s.routes().ServeHTTP(response, request)
	var receipt Receipt
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || receipt.RequestedPromotionCode != nil {
		t.Fatalf("undecodable input was reported as captured: %+v", receipt)
	}
	if err := json.Unmarshal([]byte(`{"observation_id":"older-observation"}`), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.RequestedPromotionCode != nil {
		t.Fatal("older observation acquired an invented promotion")
	}
}

func TestAmbiguousCommitReconcilesWithoutSecondWrite(t *testing.T) {
	s, m := testService()
	m.commitErr = errors.New("connection lost after commit")
	body := CheckoutRequest{OrderID: "order-2", SKU: "MUG-350", Quantity: 1}
	w, r := submit(t, s, body)
	if w.Code != 503 || r.ErrorCode != "order_commit_unconfirmed" {
		t.Fatalf("unconfirmed commit: %d %+v", w.Code, r)
	}
	m.commitErr = nil
	w, r = submit(t, s, body)
	if w.Code != 200 || len(m.orders) != 1 || r.LedgerWrites == nil || *r.LedgerWrites != 1 {
		t.Fatalf("retry did not reconcile: %d %+v", w.Code, r)
	}
}

func TestObservationFailureDoesNotUndoOrder(t *testing.T) {
	s, m := testService()
	m.observationErr = errors.New("telemetry store unavailable")
	w, r := submit(t, s, CheckoutRequest{OrderID: "order-3", SKU: "COFFEE-250", Quantity: 1})
	if w.Code != 201 || len(m.orders) != 1 || r.ErrorCode != "" {
		t.Fatalf("observation failure changed business response: %d %+v", w.Code, r)
	}
}
