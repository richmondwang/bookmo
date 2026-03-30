package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Service holds business logic for the payments module.
type Service struct {
	repo   *Repository
	client *PayMongoClient
}

// NewService constructs a Service.
func NewService(repo *Repository, client *PayMongoClient) *Service {
	return &Service{repo: repo, client: client}
}

// methodType returns 'auth_capture' for cards or 'immediate_capture' for e-wallets.
func methodType(method string) string {
	switch method {
	case "gcash", "maya", "bank_transfer":
		return "immediate_capture"
	default: // card
		return "auth_capture"
	}
}

// CreateIntent creates a payment intent for a booking.
// For auth_capture (cards): authorizes the hold via PayMongo and stores status='authorized'.
// For immediate_capture (GCash/Maya): marks the intent as captured immediately.
func (s *Service) CreateIntent(ctx context.Context, req *CreateIntentRequest) (*PaymentIntent, error) {
	mt := methodType(req.Method)

	paymongoID, err := s.client.Authorize(ctx, req.AmountCentavos, req.Method, "PHP")
	if err != nil {
		return nil, fmt.Errorf("payments.CreateIntent: %w", err)
	}

	method := req.Method
	methodTypeVal := mt

	pi := &PaymentIntent{
		BookingID:      req.BookingID,
		PayMongoID:     paymongoID,
		AmountCentavos: req.AmountCentavos,
		Currency:       "PHP",
		Method:         &method,
		MethodType:     &methodTypeVal,
	}

	switch mt {
	case "immediate_capture":
		pi.Status = "captured"
		now := time.Now().UTC()
		pi.CapturedAt = &now
	default:
		pi.Status = "authorized"
	}

	if err := s.repo.CreatePaymentIntent(ctx, pi); err != nil {
		return nil, fmt.Errorf("payments.CreateIntent: %w", err)
	}
	return pi, nil
}

// HandleWebhook verifies the webhook signature, deduplicates the event, and routes it.
func (s *Service) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	if err := s.client.VerifyWebhookSignature(rawBody, signature); err != nil {
		return fmt.Errorf("payments.HandleWebhook: %w", err)
	}

	var event PayMongoEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		return fmt.Errorf("payments.HandleWebhook: parse event: %w", err)
	}

	processed, err := s.repo.IsWebhookEventProcessed(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("payments.HandleWebhook: %w", err)
	}
	if processed {
		return ErrDuplicateWebhookEvent
	}

	if err := s.repo.MarkWebhookEventProcessed(ctx, event.ID, event.Type); err != nil {
		return fmt.Errorf("payments.HandleWebhook: mark processed: %w", err)
	}

	switch event.Type {
	case "payment.paid":
		return s.handlePaymentPaid(ctx, event)
	}
	// Unknown event types are silently ignored after being recorded.
	return nil
}

// handlePaymentPaid updates the payment intent status to 'captured' when PayMongo confirms payment.
func (s *Service) handlePaymentPaid(ctx context.Context, event PayMongoEvent) error {
	paymongoID, _ := event.Data.Attributes["id"].(string)
	if paymongoID == "" {
		// Fallback: the top-level event ID is sometimes the payment intent ID.
		paymongoID = event.ID
	}

	pi, err := s.repo.GetByPayMongoID(ctx, paymongoID)
	if err != nil {
		return fmt.Errorf("payments.handlePaymentPaid: %w", err)
	}

	now := time.Now().UTC()
	status := "captured"
	paymongoStatus := "paid"
	if err := s.repo.UpdatePaymentIntentStatus(ctx, pi.ID, status, &now, nil, &paymongoStatus); err != nil {
		return fmt.Errorf("payments.handlePaymentPaid: %w", err)
	}
	return nil
}

// CapturePayment captures an authorized card payment on owner approval.
// For immediate_capture intents the funds are already captured; this is a no-op status update.
func (s *Service) CapturePayment(ctx context.Context, bookingID string) error {
	pi, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("payments.CapturePayment: %w", err)
	}

	if pi.MethodType != nil && *pi.MethodType == "immediate_capture" {
		// Already captured; nothing to do.
		return nil
	}

	if err := s.client.Capture(ctx, pi.PayMongoID, pi.AmountCentavos); err != nil {
		return fmt.Errorf("payments.CapturePayment: %w: %w", ErrCaptureFailed, err)
	}

	now := time.Now().UTC()
	paymongoStatus := "captured"
	if err := s.repo.UpdatePaymentIntentStatus(ctx, pi.ID, "captured", &now, nil, &paymongoStatus); err != nil {
		return fmt.Errorf("payments.CapturePayment: %w", err)
	}
	return nil
}

// VoidPayment voids an authorized card payment on rejection or timeout.
// For immediate_capture intents it delegates to RefundPayment instead (per ADR-001).
func (s *Service) VoidPayment(ctx context.Context, bookingID string) error {
	pi, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("payments.VoidPayment: %w", err)
	}

	if pi.MethodType != nil && *pi.MethodType == "immediate_capture" {
		// GCash/Maya cannot be voided — issue a refund instead.
		return s.RefundPayment(ctx, bookingID, "system_timeout")
	}

	if err := s.client.Void(ctx, pi.PayMongoID); err != nil {
		return fmt.Errorf("payments.VoidPayment: %w: %w", ErrVoidFailed, err)
	}

	now := time.Now().UTC()
	paymongoStatus := "voided"
	if err := s.repo.UpdatePaymentIntentStatus(ctx, pi.ID, "voided", nil, &now, &paymongoStatus); err != nil {
		return fmt.Errorf("payments.VoidPayment: %w", err)
	}
	return nil
}

// RefundPayment issues a refund for a captured payment and records it in the refunds table.
func (s *Service) RefundPayment(ctx context.Context, bookingID, reason string) error {
	pi, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("payments.RefundPayment: %w", err)
	}

	refundID, err := s.client.Refund(ctx, pi.PayMongoID, pi.AmountCentavos, reason)
	if err != nil {
		return fmt.Errorf("payments.RefundPayment: %w: %w", ErrRefundFailed, err)
	}

	reasonVal := reason
	ref := &Refund{
		PaymentIntentID:  pi.ID,
		BookingID:        bookingID,
		PayMongoRefundID: &refundID,
		AmountCentavos:   pi.AmountCentavos,
		Reason:           &reasonVal,
		Status:           "succeeded",
	}
	if err := s.repo.CreateRefund(ctx, ref); err != nil {
		return fmt.Errorf("payments.RefundPayment: create refund record: %w", err)
	}

	paymongoStatus := "refunded"
	if err := s.repo.UpdatePaymentIntentStatus(ctx, pi.ID, "refunded", nil, nil, &paymongoStatus); err != nil {
		return fmt.Errorf("payments.RefundPayment: %w", err)
	}
	return nil
}
