package billing_paddle

import "time"

type TestSubscriptionCreatedPayload struct {
	EventID     string
	SubID       string
	CustomerID  string
	DatabaseID  string
	QuantityGB  int
	PeriodStart time.Time
	PeriodEnd   time.Time
}

type TestSubscriptionUpdatedPayload struct {
	EventID               string
	SubID                 string
	CustomerID            string
	QuantityGB            int
	PeriodStart           time.Time
	PeriodEnd             time.Time
	HasScheduledChange    bool
	ScheduledChangeAction string
}

type TestSubscriptionCanceledPayload struct {
	EventID    string
	SubID      string
	CustomerID string
}

type TestTransactionCompletedPayload struct {
	EventID     string
	TxnID       string
	SubID       string
	CustomerID  string
	TotalCents  int64
	QuantityGB  int
	PeriodStart time.Time
	PeriodEnd   time.Time
}

type TestSubscriptionPastDuePayload struct {
	EventID     string
	SubID       string
	CustomerID  string
	QuantityGB  int
	PeriodStart time.Time
	PeriodEnd   time.Time
}
