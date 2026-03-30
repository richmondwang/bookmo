package payments

import "time"

// PaymentIntent is the DB row for a payment_intents record.
type PaymentIntent struct {
	ID             string     `db:"id"`
	BookingID      string     `db:"booking_id"`
	PayMongoID     string     `db:"paymongo_id"`
	AmountCentavos int        `db:"amount_centavos"`
	Currency       string     `db:"currency"`
	Method         *string    `db:"method"`
	MethodType     *string    `db:"method_type"`
	Status         string     `db:"status"`
	PayMongoStatus *string    `db:"paymongo_status"`
	CapturedAt     *time.Time `db:"captured_at"`
	VoidedAt       *time.Time `db:"voided_at"`
	CreatedAt      time.Time  `db:"created_at"`
}

// Refund is the DB row for a refunds record.
type Refund struct {
	ID               string    `db:"id"`
	PaymentIntentID  string    `db:"payment_intent_id"`
	BookingID        string    `db:"booking_id"`
	PayMongoRefundID *string   `db:"paymongo_refund_id"`
	AmountCentavos   int       `db:"amount_centavos"`
	Reason           *string   `db:"reason"`
	Status           string    `db:"status"`
	CreatedAt        time.Time `db:"created_at"`
}

// WebhookEvent is the DB row for a webhook_events record.
type WebhookEvent struct {
	ID          string    `db:"id"`
	Type        string    `db:"type"`
	ProcessedAt time.Time `db:"processed_at"`
}

// CreateIntentRequest is the HTTP request body for POST /payments/intent.
type CreateIntentRequest struct {
	BookingID      string `json:"booking_id" binding:"required"`
	AmountCentavos int    `json:"amount_centavos" binding:"required,min=1"`
	Method         string `json:"method" binding:"required"`
}

// PayMongoEventData holds the attributes payload of a webhook event.
type PayMongoEventData struct {
	Attributes map[string]any `json:"attributes"`
}

// PayMongoEvent is a PayMongo webhook event.
type PayMongoEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Attributes map[string]any `json:"attributes"`
	} `json:"data"`
}

// PaymentIntentResponse is the response DTO for a payment intent.
type PaymentIntentResponse struct {
	ID             string  `json:"id"`
	BookingID      string  `json:"booking_id"`
	PayMongoID     string  `json:"paymongo_id"`
	AmountCentavos int     `json:"amount_centavos"`
	Currency       string  `json:"currency"`
	Method         *string `json:"method,omitempty"`
	MethodType     *string `json:"method_type,omitempty"`
	Status         string  `json:"status"`
	CheckoutURL    *string `json:"checkout_url,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// toResponse converts a PaymentIntent DB row to its response DTO.
func toResponse(pi *PaymentIntent) PaymentIntentResponse {
	return PaymentIntentResponse{
		ID:             pi.ID,
		BookingID:      pi.BookingID,
		PayMongoID:     pi.PayMongoID,
		AmountCentavos: pi.AmountCentavos,
		Currency:       pi.Currency,
		Method:         pi.Method,
		MethodType:     pi.MethodType,
		Status:         pi.Status,
		CreatedAt:      pi.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
