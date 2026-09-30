package main

func checkout(receipt *Receipt) ledger {
	orderTotal := receipt.UnitPrice*receipt.Quantity - receipt.Discount
	return ledger{Total: orderTotal, Writes: 1}
}
