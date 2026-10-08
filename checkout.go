package main

func checkout(receipt *Receipt) ledger {
	subtotal := receipt.UnitPrice * receipt.Quantity
	return ledger{Total: subtotal - receipt.Discount, Writes: 1}
}
