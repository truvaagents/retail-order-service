package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/truvaagents/truva-g3/core"
	"github.com/truvaagents/truva-g3/telemetry"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type service struct {
	store  orderStore
	logger core.Logger
	token  string
	build  BuildInfo
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if s.store.Ready(ctx) != nil {
			http.Error(w, "store unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /catalog", func(w http.ResponseWriter, r *http.Request) { WriteJSON(w, http.StatusOK, catalog) })
	mux.HandleFunc("POST /orders", s.createOrder)
	mux.HandleFunc("GET /orders/{id}", s.order)
	mux.HandleFunc("GET /evidence/{id}", s.evidence)
	mux.HandleFunc("GET /observations", s.observations)
	mux.HandleFunc("GET /build-info", s.buildInfo)
	return mux
}
func requestDigest(request CheckoutRequest) string {
	identity := struct {
		OrderID  string `json:"order_id"`
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	}{OrderID: request.OrderID, SKU: request.SKU, Quantity: request.Quantity}
	data, _ := json.Marshal(identity)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *service) createOrder(w http.ResponseWriter, r *http.Request) {
	if !Authorized(r, s.token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := r.Context()
	receipt := Receipt{ObservationID: rand.Text(), Operation: "create_order", At: time.Now().UTC(), Revision: s.build.Revision, TraceID: telemetry.GetTraceContext(ctx).TraceID}
	ctx = core.WithRequestID(ctx, receipt.ObservationID)
	ctx = telemetry.WithBaggage(ctx, "request_id", receipt.ObservationID)
	w.Header().Set("X-Request-ID", receipt.ObservationID)
	var request CheckoutRequest
	if ReadJSON(w, r, &request) != nil {
		s.complete(ctx, w, receipt, http.StatusBadRequest, "invalid_json")
		return
	}
	if !identifier.MatchString(request.OrderID) {
		s.complete(ctx, w, receipt, http.StatusUnprocessableEntity, "invalid_order")
		return
	}
	receipt.OrderID, receipt.SKU, receipt.Quantity = request.OrderID, request.SKU, request.Quantity
	digest := requestDigest(request)
	// Check request identity before today's catalogue so an accepted order can
	// still be replayed after prices or promotion availability change.
	previous, err := s.store.ReadOrder(ctx, request.OrderID)
	if err == nil {
		s.replay(ctx, w, receipt, previous, digest)
		return
	}
	if !errors.Is(err, errNotFound) {
		s.complete(ctx, w, receipt, http.StatusServiceUnavailable, "order_store_unavailable")
		return
	}
	product, discount, err := priceRequest(request)
	if err != nil {
		s.complete(ctx, w, receipt, http.StatusUnprocessableEntity, err.Error())
		return
	}
	receipt.UnitPrice, receipt.Discount, receipt.Currency = product.UnitPrice, discount, product.Currency
	receipt.HTTPStatus = http.StatusCreated
	receipt.OrderRevision = receipt.Revision
	order := Order{Request: request, RequestSHA256: digest, Receipt: receipt, Ledger: checkout(&receipt)}
	created, err := s.store.CreateOrder(ctx, order)
	if err != nil {
		// A transport failure may follow a successful commit. The same order ID
		// reconciles this uncertainty; do not claim rollback or retry blindly.
		s.complete(ctx, w, receipt, http.StatusServiceUnavailable, "order_commit_unconfirmed")
		return
	}
	if !created {
		previous, err := s.store.ReadOrder(ctx, request.OrderID)
		if err != nil {
			s.complete(ctx, w, receipt, http.StatusServiceUnavailable, "order_store_unavailable")
			return
		}
		s.replay(ctx, w, receipt, previous, digest)
		return
	}
	receipt.LedgerTotal, receipt.LedgerWrites = &order.Ledger.Total, &order.Ledger.Writes
	s.complete(ctx, w, receipt, http.StatusCreated, "")
}
func (s *service) replay(ctx context.Context, w http.ResponseWriter, receipt Receipt, previous Order, digest string) {
	if previous.RequestSHA256 != digest {
		s.complete(ctx, w, receipt, http.StatusConflict, "order_id_conflict")
		return
	}
	receipt.UnitPrice, receipt.Discount, receipt.Currency = previous.Receipt.UnitPrice, previous.Receipt.Discount, previous.Receipt.Currency
	receipt.OrderRevision = previous.Receipt.Revision
	receipt.LedgerTotal, receipt.LedgerWrites = &previous.Ledger.Total, &previous.Ledger.Writes
	w.Header().Set("Idempotency-Replayed", "true")
	s.complete(ctx, w, receipt, http.StatusOK, "")
}
func (s *service) complete(ctx context.Context, w http.ResponseWriter, receipt Receipt, status int, code string) {
	receipt.HTTPStatus, receipt.ErrorCode = status, code
	fields := map[string]interface{}{"observation_id": receipt.ObservationID, "order_id": receipt.OrderID, "operation": receipt.Operation, "http_status": status, "revision": receipt.Revision, "error_code": code}
	switch {
	case status >= 500:
		s.logger.ErrorWithContext(ctx, "Order request failed", fields)
	case status >= 400:
		s.logger.WarnWithContext(ctx, "Order request rejected", fields)
	default:
		s.logger.InfoWithContext(ctx, "Order request completed", fields)
	}
	// Telemetry failure cannot undo a committed order or turn its business
	// response into an apparent failed checkout.
	if err := s.store.SaveReceipt(ctx, receipt); err != nil {
		s.logger.WarnWithContext(ctx, "Order observation unavailable", map[string]interface{}{"operation": "record_observation", "status": "receipt_write_failed"})
	} else if err := s.store.Index(ctx, receipt); err != nil {
		s.logger.WarnWithContext(ctx, "Order observation index incomplete", map[string]interface{}{"operation": "record_observation", "status": "index_write_failed"})
	}
	WriteJSON(w, status, receipt)
}
func (s *service) order(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !identifier.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	order, err := s.store.ReadOrder(r.Context(), id)
	if errors.Is(err, errNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "order store unavailable", http.StatusServiceUnavailable)
		return
	}
	WriteJSON(w, http.StatusOK, order)
}
func (s *service) evidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !identifier.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	receipt, err := s.store.ReadReceipt(r.Context(), id)
	if errors.Is(err, errNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "observation store unavailable", http.StatusServiceUnavailable)
		return
	}
	// Read the business outcome independently of the returned HTTP receipt.
	// Rejected attempts must not inherit a different request's accepted order.
	receipt.LedgerTotal, receipt.LedgerWrites = nil, nil
	receipt.OrderRevision = ""
	if receipt.HTTPStatus >= 200 && receipt.HTTPStatus < 300 {
		order, err := s.store.ReadOrder(r.Context(), receipt.OrderID)
		if err != nil && !errors.Is(err, errNotFound) {
			http.Error(w, "order store unavailable", http.StatusServiceUnavailable)
			return
		}
		if err == nil {
			receipt.OrderRevision = order.Receipt.Revision
			receipt.LedgerTotal, receipt.LedgerWrites = &order.Ledger.Total, &order.Ledger.Writes
		}
	}
	WriteJSON(w, http.StatusOK, receipt)
}
func (s *service) observations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, startErr := time.Parse(time.RFC3339Nano, q.Get("start"))
	end, endErr := time.Parse(time.RFC3339Nano, q.Get("end"))
	limit, limitErr := strconv.Atoi(q.Get("limit"))
	if len(q) != 3 || len(q["start"]) != 1 || len(q["end"]) != 1 || len(q["limit"]) != 1 || startErr != nil || endErr != nil || limitErr != nil || limit < 1 || limit > 50 || !start.Before(end) || end.Sub(start) > 10*time.Minute || time.Since(start) > 24*time.Hour || end.After(time.Now()) {
		http.Error(w, "invalid observation window or limit", http.StatusBadRequest)
		return
	}
	feed, err := s.store.ReadFeed(r.Context(), Window{Start: start, End: end}, limit)
	if err != nil {
		http.Error(w, "observation index unavailable", http.StatusServiceUnavailable)
		return
	}
	WriteJSON(w, http.StatusOK, feed)
}
