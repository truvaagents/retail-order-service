package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestCatalogSearch(t *testing.T) {
	tests := []struct {
		name, query string
		want        []string
		total, next string
	}{
		{"description and price", "q=COFFEE&min_price_cents=700&max_price_cents=1000&sort=price_asc", []string{"FILTER-100", "COFFEE-250"}, "2", ""},
		{"partial SKU", "q=g-35", []string{"MUG-350"}, "1", ""},
		{"first page", "sort=price_desc&limit=2", []string{"MUG-350", "COFFEE-250"}, "5", "2"},
		{"middle page", "sort=price_desc&limit=2&offset=2", []string{"TEA-100", "FILTER-100"}, "5", "4"},
		{"last page", "sort=price_desc&limit=2&offset=4", []string{"CHOCOLATE-80"}, "5", ""},
		{"beyond end", "offset=9", []string{}, "5", ""},
		{"zero price ceiling", "max_price_cents=0", []string{}, "0", ""},
	}
	s, _ := testService()
	handler := s.routes()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/catalog?"+tc.query, nil))
			var items []Product
			if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil || items == nil {
				t.Fatalf("invalid product list %q: %v", w.Body.String(), err)
			}
			var skus []string
			for _, item := range items {
				skus = append(skus, item.SKU)
			}
			if w.Code != http.StatusOK || !slices.Equal(skus, tc.want) || w.Header().Get("X-Total-Count") != tc.total || w.Header().Get("X-Next-Offset") != tc.next {
				t.Fatalf("unexpected page: status=%d SKUs=%v headers=%v", w.Code, skus, w.Header())
			}
		})
	}
}

func TestCatalogRejectsInvalidFilters(t *testing.T) {
	s, _ := testService()
	handler := s.routes()
	for _, query := range []string{"limit=0", "limit=51", "offset=-1", "limit=abc", "q=a&q=b", "min_price_cents=6&max_price_cents=5", "sort=unknown", "other=1"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/catalog?"+query, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestCatalogPriceTiesAndOriginalOrder(t *testing.T) {
	products := []Product{{SKU: "B", UnitPrice: 99}, {SKU: "A", UnitPrice: 99}}
	query := catalogQuery{sort: "price_desc"}
	got := query.selectProducts(products)
	if got[0].SKU != "A" || products[0].SKU != "B" {
		t.Fatalf("unexpected tie ordering: result=%v source=%v", got, products)
	}
}
