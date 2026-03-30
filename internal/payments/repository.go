package payments

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the payments module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreatePaymentIntent inserts a new payment_intents row and populates id and created_at.
func (r *Repository) CreatePaymentIntent(ctx context.Context, pi *PaymentIntent) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO payment_intents
			(booking_id, paymongo_id, amount_centavos, currency, method, method_type, status, paymongo_status, captured_at, voided_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`,
		pi.BookingID,
		pi.PayMongoID,
		pi.AmountCentavos,
		pi.Currency,
		pi.Method,
		pi.MethodType,
		pi.Status,
		pi.PayMongoStatus,
		pi.CapturedAt,
		pi.VoidedAt,
	)
	return row.Scan(&pi.ID, &pi.CreatedAt)
}

// GetByBookingID returns the payment intent for a booking or ErrPaymentNotFound.
func (r *Repository) GetByBookingID(ctx context.Context, bookingID string) (*PaymentIntent, error) {
	pi := &PaymentIntent{}
	err := r.db.QueryRow(ctx, `
		SELECT id, booking_id, paymongo_id, amount_centavos, currency, method, method_type,
		       status, paymongo_status, captured_at, voided_at, created_at
		FROM payment_intents
		WHERE booking_id = $1`, bookingID).
		Scan(
			&pi.ID, &pi.BookingID, &pi.PayMongoID, &pi.AmountCentavos, &pi.Currency,
			&pi.Method, &pi.MethodType, &pi.Status, &pi.PayMongoStatus,
			&pi.CapturedAt, &pi.VoidedAt, &pi.CreatedAt,
		)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return pi, nil
}

// GetByPayMongoID returns the payment intent with the given PayMongo ID or ErrPaymentNotFound.
func (r *Repository) GetByPayMongoID(ctx context.Context, paymongoID string) (*PaymentIntent, error) {
	pi := &PaymentIntent{}
	err := r.db.QueryRow(ctx, `
		SELECT id, booking_id, paymongo_id, amount_centavos, currency, method, method_type,
		       status, paymongo_status, captured_at, voided_at, created_at
		FROM payment_intents
		WHERE paymongo_id = $1`, paymongoID).
		Scan(
			&pi.ID, &pi.BookingID, &pi.PayMongoID, &pi.AmountCentavos, &pi.Currency,
			&pi.Method, &pi.MethodType, &pi.Status, &pi.PayMongoStatus,
			&pi.CapturedAt, &pi.VoidedAt, &pi.CreatedAt,
		)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return pi, nil
}

// UpdatePaymentIntentStatus updates the status (and optional timestamps) on a payment intent.
func (r *Repository) UpdatePaymentIntentStatus(ctx context.Context, id, status string, capturedAt, voidedAt *time.Time, paymongoStatus *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payment_intents
		SET status = $2,
		    captured_at = COALESCE($3, captured_at),
		    voided_at = COALESCE($4, voided_at),
		    paymongo_status = COALESCE($5, paymongo_status)
		WHERE id = $1`,
		id, status, capturedAt, voidedAt, paymongoStatus,
	)
	return err
}

// CreateRefund inserts a new refunds row and populates id and created_at.
func (r *Repository) CreateRefund(ctx context.Context, ref *Refund) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO refunds
			(payment_intent_id, booking_id, paymongo_refund_id, amount_centavos, reason, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		ref.PaymentIntentID,
		ref.BookingID,
		ref.PayMongoRefundID,
		ref.AmountCentavos,
		ref.Reason,
		ref.Status,
	)
	return row.Scan(&ref.ID, &ref.CreatedAt)
}

// UpdateRefundStatus updates the status and optional paymongo_refund_id on a refund.
func (r *Repository) UpdateRefundStatus(ctx context.Context, id, status string, paymongoRefundID *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refunds
		SET status = $2,
		    paymongo_refund_id = COALESCE($3, paymongo_refund_id)
		WHERE id = $1`,
		id, status, paymongoRefundID,
	)
	return err
}

// IsWebhookEventProcessed returns true if the event has already been processed.
func (r *Repository) IsWebhookEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM webhook_events WHERE id = $1)`, eventID,
	).Scan(&exists)
	return exists, err
}

// MarkWebhookEventProcessed inserts a webhook_events row to record a processed event.
func (r *Repository) MarkWebhookEventProcessed(ctx context.Context, eventID, eventType string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO webhook_events (id, type) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
		eventID, eventType,
	)
	return err
}
