package billing_models

type InvoiceStatus string

const (
	InvoiceStatusPending  InvoiceStatus = "pending"
	InvoiceStatusPaid     InvoiceStatus = "paid"
	InvoiceStatusFailed   InvoiceStatus = "failed"
	InvoiceStatusRefunded InvoiceStatus = "refunded"
	InvoiceStatusDisputed InvoiceStatus = "disputed"
)
