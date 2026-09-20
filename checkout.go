package main

func checkout(receipt *Receipt) ledger {
	unitTotal := receipt.UnitPrice - receipt.Discount
	return ledger{Total: unitTotal * receipt.Quantity, Writes: 1}
}
