package marketplace

import (
	"context"
	"fmt"
)

// PayoutClient handles PayMongo Payouts API calls.
// All methods are stubs — replace with real PayMongo HTTP calls before going live.
type PayoutClient struct {
	secretKey string
}

// NewPayoutClient creates a PayoutClient.
func NewPayoutClient(secretKey string) *PayoutClient {
	return &PayoutClient{secretKey: secretKey}
}

// InitiatePayout creates a PayMongo payout transfer and returns the PayMongo payout ID.
// Stub implementation: returns a fake ID. Real implementation should POST to
// https://api.paymongo.com/v1/payouts with the account details and amount.
func (c *PayoutClient) InitiatePayout(ctx context.Context, accountType, accountNumber, accountName string, amountCentavos int) (string, error) {
	if c.secretKey == "" {
		return "", fmt.Errorf("marketplace.InitiatePayout: %w", ErrPayoutFailed)
	}
	// TODO: POST to PayMongo Payouts API.
	// The real call should include:
	//   amount: amountCentavos
	//   currency: "PHP"
	//   account_type: accountType (gcash|maya|bank_transfer)
	//   account_number: accountNumber
	//   account_name: accountName
	// And return the response payout ID.
	stubID := fmt.Sprintf("pmg_payout_stub_%s_%d", accountType, amountCentavos)
	return stubID, nil
}

// GetPayoutStatus returns the current status of a PayMongo payout.
// Stub returns "paid" for any ID.
func (c *PayoutClient) GetPayoutStatus(ctx context.Context, payoutID string) (string, error) {
	// TODO: GET https://api.paymongo.com/v1/payouts/{id} and return attributes.status
	return "paid", nil
}
