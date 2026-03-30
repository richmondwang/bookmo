package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// PayMongoClient wraps the PayMongo API.
type PayMongoClient struct {
	secretKey     string
	webhookSecret string
}

// NewPayMongoClient constructs a PayMongoClient.
func NewPayMongoClient(secretKey, webhookSecret string) *PayMongoClient {
	return &PayMongoClient{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
	}
}

// VerifyWebhookSignature verifies the HMAC-SHA256 signature on a PayMongo webhook.
// Header format: "t=<timestamp>,te=<sig>" (test) or "t=<timestamp>,li=<sig>" (live)
// Signed payload: "<timestamp>.<raw_body>"
func (c *PayMongoClient) VerifyWebhookSignature(rawBody []byte, signatureHeader string) error {
	var timestamp, sig string

	for _, part := range strings.Split(signatureHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "te", "li":
			sig = kv[1]
		}
	}

	if timestamp == "" || sig == "" {
		return ErrInvalidWebhookSignature
	}

	payload := timestamp + "." + string(rawBody)
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

// Authorize creates a payment intent (card auth hold). Returns the PayMongo ID.
// MVP stub: returns a generated ID without hitting the PayMongo API.
func (c *PayMongoClient) Authorize(ctx context.Context, amountCentavos int, method, currency string) (string, error) {
	return fmt.Sprintf("pi_%d", time.Now().UnixNano()), nil
}

// Capture captures an authorized payment.
// MVP stub: returns nil.
func (c *PayMongoClient) Capture(ctx context.Context, paymongoID string, amountCentavos int) error {
	return nil
}

// Void cancels an authorized hold (cards only).
// MVP stub: returns nil.
func (c *PayMongoClient) Void(ctx context.Context, paymongoID string) error {
	return nil
}

// Refund issues a refund and returns the PayMongo refund ID.
// MVP stub: returns a generated ID without hitting the PayMongo API.
func (c *PayMongoClient) Refund(ctx context.Context, paymongoID string, amountCentavos int, reason string) (string, error) {
	return fmt.Sprintf("re_%d", time.Now().UnixNano()), nil
}
