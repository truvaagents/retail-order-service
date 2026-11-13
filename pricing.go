package main

import (
	"errors"
	"strings"
)

type volumeTier struct {
	MinimumQuantity int
	Percent         int
}

type promotion struct {
	FixedCents      int
	Percent         int
	MinimumSubtotal int
	MaximumDiscount int
	SKUPrefix       string
}

var volumeTiers = []volumeTier{
	{MinimumQuantity: 10, Percent: 5},
	{MinimumQuantity: 25, Percent: 10},
}

var promotions = map[string]promotion{
	"WELCOME100": {FixedCents: 100},
	"SAVE10":     {Percent: 10, MinimumSubtotal: 5000, MaximumDiscount: 1000},
	"MUG15":      {Percent: 15, SKUPrefix: "MUG-"},
}

func percentageDiscount(amount, percent int) int {
	return amount * percent / 100
}

func orderDiscount(product Product, quantity int, code string) (int, error) {
	subtotal := product.UnitPrice * quantity
	volumePercent := 0
	for _, tier := range volumeTiers {
		if quantity >= tier.MinimumQuantity {
			volumePercent = tier.Percent
		}
	}
	discount := percentageDiscount(subtotal, volumePercent)
	if code == "" {
		return discount, nil
	}
	selected, ok := promotions[code]
	if !ok {
		return 0, errors.New("unknown_promotion")
	}
	if subtotal < selected.MinimumSubtotal || !strings.HasPrefix(product.SKU, selected.SKUPrefix) {
		return 0, errors.New("promotion_not_eligible")
	}
	discount += selected.FixedCents + percentageDiscount(subtotal-discount, selected.Percent)
	if selected.MaximumDiscount > 0 && discount > selected.MaximumDiscount {
		discount = selected.MaximumDiscount
	}
	return min(discount, subtotal), nil
}
