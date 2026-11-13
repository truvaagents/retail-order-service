package main

func checkout(receipt *Receipt) ledger {
	return ledger{Total: receipt.UnitPrice*receipt.Quantity - receipt.Discount, Writes: 1}
}
