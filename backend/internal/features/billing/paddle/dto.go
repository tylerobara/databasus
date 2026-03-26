package billing_paddle

import "encoding/json"

type PaddleWebhookDTO struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Data      json.RawMessage
}
