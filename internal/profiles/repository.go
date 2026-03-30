package profiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the profiles module.
// No business logic lives here.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetProfile fetches the full user row by ID.
func (r *Repository) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	const q = `
		SELECT id, email, phone, role, full_name, bio, profile_photo_url, is_verified, created_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.db.QueryRow(ctx, q, userID)
	p := &UserProfile{}
	err := row.Scan(
		&p.ID, &p.Email, &p.Phone, &p.Role,
		&p.FullName, &p.Bio, &p.ProfilePhotoURL,
		&p.IsVerified, &p.CreatedAt, &p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("profiles.GetProfile: %w", err)
	}
	return p, nil
}

// UpdateProfile applies a partial update to the authenticated user's profile.
// Only non-nil fields in req are updated; existing values are kept via COALESCE.
func (r *Repository) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) error {
	const q = `
		UPDATE users
		SET
			full_name = COALESCE($2, full_name),
			bio       = COALESCE($3, bio),
			phone     = COALESCE($4, phone)
		WHERE id = $1 AND deleted_at IS NULL`

	ct, err := r.db.Exec(ctx, q, userID, req.FullName, req.Bio, req.Phone)
	if err != nil {
		return fmt.Errorf("profiles.UpdateProfile: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrProfileNotFound
	}
	return nil
}

// SetPhotoURL persists the confirmed CDN URL to users.profile_photo_url.
func (r *Repository) SetPhotoURL(ctx context.Context, userID, cdnURL string) error {
	const q = `UPDATE users SET profile_photo_url = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, cdnURL)
	if err != nil {
		return fmt.Errorf("profiles.SetPhotoURL: %w", err)
	}
	return nil
}

// GetTrustProfile fetches the pre-aggregated trust row for a customer.
// This table is trigger-owned — application code only reads it.
func (r *Repository) GetTrustProfile(ctx context.Context, customerID string) (*CustomerTrustProfile, error) {
	const q = `
		SELECT customer_id, total_bookings, completed_bookings, cancelled_bookings,
		       completion_rate, cancellation_rate, avg_owner_rating, total_owner_reviews, last_updated
		FROM customer_trust_profiles
		WHERE customer_id = $1`

	row := r.db.QueryRow(ctx, q, customerID)
	t := &CustomerTrustProfile{}
	err := row.Scan(
		&t.CustomerID, &t.TotalBookings, &t.CompletedBookings, &t.CancelledBookings,
		&t.CompletionRate, &t.CancellationRate, &t.AvgOwnerRating, &t.TotalOwnerReviews,
		&t.LastUpdated,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("profiles.GetTrustProfile: %w", err)
	}
	return t, nil
}

// GetOwnerReviewsForCustomer returns the 20 most recent published owner reviews for a customer.
func (r *Repository) GetOwnerReviewsForCustomer(ctx context.Context, customerID string) ([]OwnerReviewOnProfile, error) {
	const q = `
		SELECT id, owner_id, rating, body, submitted_at
		FROM customer_reviews
		WHERE customer_id = $1 AND status = 'published'
		ORDER BY submitted_at DESC
		LIMIT 20`

	rows, err := r.db.Query(ctx, q, customerID)
	if err != nil {
		return nil, fmt.Errorf("profiles.GetOwnerReviewsForCustomer: %w", err)
	}
	defer rows.Close()

	var reviews []OwnerReviewOnProfile
	for rows.Next() {
		var rv OwnerReviewOnProfile
		if err := rows.Scan(&rv.ID, &rv.OwnerID, &rv.Rating, &rv.Body, &rv.SubmittedAt); err != nil {
			return nil, fmt.Errorf("profiles.GetOwnerReviewsForCustomer scan: %w", err)
		}
		reviews = append(reviews, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profiles.GetOwnerReviewsForCustomer rows: %w", err)
	}
	return reviews, nil
}
