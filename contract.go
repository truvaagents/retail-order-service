package main

import "time"

type CheckoutRequest struct {
	OrderID       string `json:"order_id"`
	SKU           string `json:"sku"`
	Quantity      int    `json:"quantity"`
	PromotionCode string `json:"promotion_code,omitempty"`
}

type Product struct {
	SKU              string `json:"sku"`
	Name             string `json:"name"`
	UnitPrice        int    `json:"unit_price_cents"`
	Currency         string `json:"currency"`
	MaxOrderQuantity int    `json:"max_order_quantity"`
}

type Receipt struct {
	ObservationID string    `json:"observation_id"`
	OrderID       string    `json:"order_id"`
	Operation     string    `json:"operation"`
	At            time.Time `json:"at"`
	Revision      string    `json:"revision"`
	OrderRevision string    `json:"order_revision,omitempty"`
	TraceID       string    `json:"target_trace_id"`
	HTTPStatus    int       `json:"http_status"`
	ErrorCode     string    `json:"error_code,omitempty"`
	SKU           string    `json:"sku"`
	UnitPrice     int       `json:"unit_price_cents"`
	Quantity      int       `json:"quantity"`
	Discount      int       `json:"discount_cents"`
	Currency      string    `json:"currency"`
	LedgerTotal   *int      `json:"ledger_total_cents,omitempty"`
	LedgerWrites  *int      `json:"ledger_writes,omitempty"`
}

type ledger struct {
	Total  int `json:"total"`
	Writes int `json:"writes"`
}

// Order is immutable: the request identity, pricing snapshot and accepted
// outcome are committed together and cannot be overwritten by a retry.
type Order struct {
	Request       CheckoutRequest `json:"request"`
	RequestSHA256 string          `json:"request_sha256"`
	Receipt       Receipt         `json:"receipt"`
	Ledger        ledger          `json:"ledger"`
}

type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type Candidate struct {
	ObservationID string    `json:"observation_id"`
	Operation     string    `json:"operation"`
	At            time.Time `json:"at"`
	HTTPStatus    int       `json:"http_status"`
	ErrorCode     string    `json:"error_code,omitempty"`
}

// Feed is an advisory execution index, not a transactional business outbox.
type Feed struct {
	Window     Window      `json:"window"`
	Candidates []Candidate `json:"candidates"`
	Truncated  bool        `json:"truncated"`
}
