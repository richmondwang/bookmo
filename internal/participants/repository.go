package participants

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the participants module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// resolveEligibility applies the ADR-009 priority chain to determine whether
// participants are allowed on a service. NULL means "not set, check next".
func resolveEligibility(serviceOverride, categoryValue, parentValue *bool) bool {
	if serviceOverride != nil {
		return *serviceOverride
	}
	if categoryValue != nil {
		return *categoryValue
	}
	if parentValue != nil {
		return *parentValue
	}
	return false
}

// ResolveParticipantEligibility runs the eligibility query for the given booking
// and resolves the effective allows_participants value per ADR-009.
func (r *Repository) ResolveParticipantEligibility(ctx context.Context, bookingID string) (bool, error) {
	const q = `
		SELECT
			s.allows_participants  AS service_override,
			c.allows_participants  AS category_value,
			cp.allows_participants AS parent_value
		FROM bookings b
		JOIN services s    ON s.id = b.service_id
		JOIN categories c  ON c.id = s.category_id
		LEFT JOIN categories cp ON cp.id = c.parent_id
		WHERE b.id = $1
	`
	var serviceOverride, categoryValue, parentValue *bool
	err := r.db.QueryRow(ctx, q, bookingID).Scan(&serviceOverride, &categoryValue, &parentValue)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrParticipantNotFound
		}
		return false, fmt.Errorf("participants.ResolveParticipantEligibility: %w", err)
	}
	return resolveEligibility(serviceOverride, categoryValue, parentValue), nil
}

// GetBookingForParticipant retrieves the customer_id and status of a booking.
func (r *Repository) GetBookingForParticipant(ctx context.Context, bookingID string) (customerID, status string, err error) {
	const q = `
		SELECT customer_id, status
		FROM bookings
		WHERE id = $1 AND deleted_at IS NULL
	`
	err = r.db.QueryRow(ctx, q, bookingID).Scan(&customerID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrParticipantNotFound
		}
		return "", "", fmt.Errorf("participants.GetBookingForParticipant: %w", err)
	}
	return customerID, status, nil
}

// Invite inserts a new booking_participants row. Detects the unique constraint
// violation (23505) and returns ErrAlreadyInvited in that case.
func (r *Repository) Invite(ctx context.Context, p *BookingParticipant) error {
	const q = `
		INSERT INTO booking_participants (booking_id, user_id, invited_by)
		VALUES ($1, $2, $3)
		RETURNING id, invited_at
	`
	err := r.db.QueryRow(ctx, q, p.BookingID, p.UserID, p.InvitedBy).Scan(&p.ID, &p.InvitedAt)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return ErrAlreadyInvited
		}
		return fmt.Errorf("participants.Invite: %w", err)
	}
	return nil
}

// GetByBooking returns all participants for the given booking, ordered by invited_at.
func (r *Repository) GetByBooking(ctx context.Context, bookingID string) ([]BookingParticipant, error) {
	const q = `
		SELECT id, booking_id, user_id, invited_by, status, invited_at, responded_at, left_at
		FROM booking_participants
		WHERE booking_id = $1
		ORDER BY invited_at
	`
	rows, err := r.db.Query(ctx, q, bookingID)
	if err != nil {
		return nil, fmt.Errorf("participants.GetByBooking: %w", err)
	}
	defer rows.Close()

	var out []BookingParticipant
	for rows.Next() {
		var p BookingParticipant
		if err := rows.Scan(
			&p.ID, &p.BookingID, &p.UserID, &p.InvitedBy,
			&p.Status, &p.InvitedAt, &p.RespondedAt, &p.LeftAt,
		); err != nil {
			return nil, fmt.Errorf("participants.GetByBooking scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("participants.GetByBooking rows: %w", err)
	}
	return out, nil
}

// GetParticipant retrieves a single participant row by (booking_id, user_id).
func (r *Repository) GetParticipant(ctx context.Context, bookingID, userID string) (*BookingParticipant, error) {
	const q = `
		SELECT id, booking_id, user_id, invited_by, status, invited_at, responded_at, left_at
		FROM booking_participants
		WHERE booking_id = $1 AND user_id = $2
	`
	var p BookingParticipant
	err := r.db.QueryRow(ctx, q, bookingID, userID).Scan(
		&p.ID, &p.BookingID, &p.UserID, &p.InvitedBy,
		&p.Status, &p.InvitedAt, &p.RespondedAt, &p.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrParticipantNotFound
		}
		return nil, fmt.Errorf("participants.GetParticipant: %w", err)
	}
	return &p, nil
}

// Accept sets status='accepted' and responded_at=now() for the given participant.
func (r *Repository) Accept(ctx context.Context, bookingID, userID string) error {
	const q = `
		UPDATE booking_participants
		SET status = 'accepted', responded_at = now()
		WHERE booking_id = $1 AND user_id = $2
	`
	tag, err := r.db.Exec(ctx, q, bookingID, userID)
	if err != nil {
		return fmt.Errorf("participants.Accept: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

// Decline sets status='declined' and responded_at=now() for the given participant.
func (r *Repository) Decline(ctx context.Context, bookingID, userID string) error {
	const q = `
		UPDATE booking_participants
		SET status = 'declined', responded_at = now()
		WHERE booking_id = $1 AND user_id = $2
	`
	tag, err := r.db.Exec(ctx, q, bookingID, userID)
	if err != nil {
		return fmt.Errorf("participants.Decline: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

// Leave sets status='left' and left_at=now() for the given participant.
func (r *Repository) Leave(ctx context.Context, bookingID, userID string) error {
	const q = `
		UPDATE booking_participants
		SET status = 'left', left_at = now()
		WHERE booking_id = $1 AND user_id = $2
	`
	tag, err := r.db.Exec(ctx, q, bookingID, userID)
	if err != nil {
		return fmt.Errorf("participants.Leave: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

// GetBookedWith returns distinct user IDs who have completed bookings with the
// given user — in both directions (creator ↔ accepted participant).
func (r *Repository) GetBookedWith(ctx context.Context, userID string) ([]BookedWithUser, error) {
	const q = `
		SELECT DISTINCT bp.user_id
		FROM booking_participants bp
		JOIN bookings b ON b.id = bp.booking_id
		WHERE b.customer_id = $1
		  AND bp.status = 'accepted'
		  AND bp.left_at IS NULL
		  AND b.status = 'completed'
		  AND b.deleted_at IS NULL

		UNION

		SELECT DISTINCT b.customer_id
		FROM booking_participants bp
		JOIN bookings b ON b.id = bp.booking_id
		WHERE bp.user_id = $1
		  AND bp.status = 'accepted'
		  AND bp.left_at IS NULL
		  AND b.status = 'completed'
		  AND b.deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("participants.GetBookedWith: %w", err)
	}
	defer rows.Close()

	var out []BookedWithUser
	for rows.Next() {
		var u BookedWithUser
		if err := rows.Scan(&u.UserID); err != nil {
			return nil, fmt.Errorf("participants.GetBookedWith scan: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("participants.GetBookedWith rows: %w", err)
	}
	return out, nil
}
