package main

func checkout(receipt *Receipt) ledger {
	total := receipt.UnitPrice*receipt.Quantity - receipt.Discount
	return ledger{Total: total, Writes: 1}
}
