package main

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	defaultCatalogPage = 3
	maxCatalogPage     = 50
)

type catalogQuery struct {
	text     string
	sort     string
	minPrice int
	maxPrice *int
	offset   int
	limit    int
}

func parseCatalogQuery(raw string) (catalogQuery, error) {
	query := catalogQuery{limit: defaultCatalogPage}
	fields, err := url.ParseQuery(raw)
	if err != nil {
		return query, errors.New("invalid catalog query")
	}
	for key, values := range fields {
		if len(values) != 1 {
			return query, errors.New("catalog filters must be supplied once")
		}
		value := values[0]
		switch key {
		case "q":
			query.text = strings.ToLower(strings.TrimSpace(value))
		case "sort":
			switch value {
			case "price_asc", "price_desc", "sku":
				query.sort = value
			default:
				return query, errors.New("unknown catalog sort order")
			}
		case "min_price_cents", "max_price_cents", "offset", "limit":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return query, errors.New(key + " must be a nonnegative integer")
			}
			switch key {
			case "min_price_cents":
				query.minPrice = n
			case "max_price_cents":
				query.maxPrice = &n
			case "offset":
				query.offset = n
			case "limit":
				if n < 1 || n > maxCatalogPage {
					return query, errors.New("limit must be between 1 and 50")
				}
				query.limit = n
			}
		default:
			return query, errors.New("unknown catalog filter")
		}
	}
	if query.maxPrice != nil && query.minPrice > *query.maxPrice {
		return query, errors.New("minimum price exceeds maximum price")
	}
	return query, nil
}

func (q catalogQuery) selectProducts(products []Product) []Product {
	matches := make([]Product, 0, len(products))
	for _, product := range products {
		if product.UnitPrice < q.minPrice || (q.maxPrice != nil && product.UnitPrice > *q.maxPrice) {
			continue
		}
		if q.text != "" && !strings.Contains(strings.ToLower(product.Name), q.text) &&
			!strings.Contains(strings.ToLower(product.SKU), q.text) {
			continue
		}
		matches = append(matches, product)
	}
	if q.sort != "" {
		slices.SortFunc(matches, q.compare)
	}
	return matches
}

// Break price ties by SKU so paging returns the same results on repeated requests.
func (q catalogQuery) compare(a, b Product) int {
	switch q.sort {
	case "price_asc":
		if difference := cmp.Compare(a.UnitPrice, b.UnitPrice); difference != 0 {
			return difference
		}
	case "price_desc":
		if difference := cmp.Compare(b.UnitPrice, a.UnitPrice); difference != 0 {
			return difference
		}
	}
	return strings.Compare(a.SKU, b.SKU)
}

func serveCatalog(w http.ResponseWriter, r *http.Request) {
	query, err := parseCatalogQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	matches := query.selectProducts(catalog)
	start := min(query.offset, len(matches))
	end := start + min(query.limit, len(matches)-start)
	w.Header().Set("X-Total-Count", strconv.Itoa(len(matches)))
	if end < len(matches) {
		w.Header().Set("X-Next-Offset", strconv.Itoa(end))
	}
	WriteJSON(w, http.StatusOK, matches[start:end])
}
