package reviews

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the reviews module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new review and populates r.ID and r.SubmittedAt via RETURNING.
// Handles the unique constraint on booking_id → ErrAlreadyReviewed.
// Does NOT write to rating_summaries — the DB trigger handles that.
func (r *Repository) Create(ctx context.Context, rev *Review) error {
	const q = `
		INSERT INTO reviews (
			booking_id, service_id, branch_id, customer_id,
			rating, body, is_anonymous, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, submitted_at`

	row := r.db.QueryRow(ctx, q,
		rev.BookingID, rev.ServiceID, rev.BranchID, rev.CustomerID,
		rev.Rating, rev.Body, rev.IsAnonymous, rev.Status,
	)
	err := row.Scan(&rev.ID, &rev.SubmittedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyReviewed
		}
		return fmt.Errorf("reviews.Create: %w", err)
	}
	return nil
}

// GetByService returns published and flagged reviews for a service, newest first, limit 50.
func (r *Repository) GetByService(ctx context.Context, serviceID string) ([]Review, error) {
	const q = `
		SELECT id, booking_id, service_id, branch_id, customer_id,
		       rating, body, is_anonymous, status, submitted_at
		FROM reviews
		WHERE service_id = $1 AND status != 'removed'
		ORDER BY submitted_at DESC
		LIMIT 50`

	rows, err := r.db.Query(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("reviews.GetByService: %w", err)
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var rev Review
		if err := rows.Scan(
			&rev.ID, &rev.BookingID, &rev.ServiceID, &rev.BranchID, &rev.CustomerID,
			&rev.Rating, &rev.Body, &rev.IsAnonymous, &rev.Status, &rev.SubmittedAt,
		); err != nil {
			return nil, fmt.Errorf("reviews.GetByService: scan: %w", err)
		}
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

// GetByID returns a single review by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Review, error) {
	const q = `
		SELECT id, booking_id, service_id, branch_id, customer_id,
		       rating, body, is_anonymous, status, submitted_at
		FROM reviews
		WHERE id = $1`

	rev := &Review{}
	err := r.db.QueryRow(ctx, q, id).Scan(
		&rev.ID, &rev.BookingID, &rev.ServiceID, &rev.BranchID, &rev.CustomerID,
		&rev.Rating, &rev.Body, &rev.IsAnonymous, &rev.Status, &rev.SubmittedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("reviews.GetByID: %w", err)
	}
	return rev, nil
}

// GetRatingSummary returns the pre-aggregated rating summary for a service.
// This row is owned by a DB trigger — never written to by application code.
func (r *Repository) GetRatingSummary(ctx context.Context, serviceID string) (*RatingSummary, error) {
	const q = `
		SELECT service_id, total_reviews, avg_rating,
		       COALESCE(five_star, 0), COALESCE(four_star, 0), COALESCE(three_star, 0),
		       COALESCE(two_star, 0), COALESCE(one_star, 0)
		FROM rating_summaries
		WHERE service_id = $1`

	s := &RatingSummary{}
	err := r.db.QueryRow(ctx, q, serviceID).Scan(
		&s.ServiceID, &s.TotalReviews, &s.AvgRating,
		&s.FiveStar, &s.FourStar, &s.ThreeStar, &s.TwoStar, &s.OneStar,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("reviews.GetRatingSummary: %w", err)
	}
	return s, nil
}

// CreateResponse inserts an owner response to a review and populates resp.ID and resp.CreatedAt.
func (r *Repository) CreateResponse(ctx context.Context, resp *ReviewResponse) error {
	const q = `
		INSERT INTO review_responses (review_id, owner_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := r.db.QueryRow(ctx, q, resp.ReviewID, resp.OwnerID, resp.Body).
		Scan(&resp.ID, &resp.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("reviews.CreateResponse: response already exists")
		}
		return fmt.Errorf("reviews.CreateResponse: %w", err)
	}
	return nil
}

// GetResponseByReview returns the owner response for a given review, if one exists.
func (r *Repository) GetResponseByReview(ctx context.Context, reviewID string) (*ReviewResponse, error) {
	const q = `
		SELECT id, review_id, owner_id, body, created_at, updated_at
		FROM review_responses
		WHERE review_id = $1`

	resp := &ReviewResponse{}
	err := r.db.QueryRow(ctx, q, reviewID).Scan(
		&resp.ID, &resp.ReviewID, &resp.OwnerID, &resp.Body,
		&resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // no response is a valid state
		}
		return nil, fmt.Errorf("reviews.GetResponseByReview: %w", err)
	}
	return resp, nil
}

// UpdateResponse updates the body and updated_at of an existing owner response.
func (r *Repository) UpdateResponse(ctx context.Context, reviewID, body string) error {
	const q = `
		UPDATE review_responses
		SET body = $2, updated_at = now()
		WHERE review_id = $1`

	ct, err := r.db.Exec(ctx, q, reviewID, body)
	if err != nil {
		return fmt.Errorf("reviews.UpdateResponse: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

// CreateFlag inserts a review flag. Handles the unique constraint → ErrAlreadyFlagged.
func (r *Repository) CreateFlag(ctx context.Context, reviewID, reportedBy, reason string) error {
	const q = `
		INSERT INTO review_flags (review_id, reported_by, reason)
		VALUES ($1, $2, $3)`

	_, err := r.db.Exec(ctx, q, reviewID, reportedBy, reason)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyFlagged
		}
		return fmt.Errorf("reviews.CreateFlag: %w", err)
	}
	return nil
}

// GetFlagCount returns the number of flags on a review.
func (r *Repository) GetFlagCount(ctx context.Context, reviewID string) (int, error) {
	const q = `SELECT COUNT(*) FROM review_flags WHERE review_id = $1`
	var count int
	if err := r.db.QueryRow(ctx, q, reviewID).Scan(&count); err != nil {
		return 0, fmt.Errorf("reviews.GetFlagCount: %w", err)
	}
	return count, nil
}

// UpdateReviewStatus sets the status of a review.
func (r *Repository) UpdateReviewStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE reviews SET status = $2 WHERE id = $1`
	ct, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("reviews.UpdateReviewStatus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

// ValidateBookingForReview checks that the booking exists, belongs to the given customer,
// has not been soft-deleted, and has status = 'completed'. Returns the booking's end_time
// so the caller can verify the review window if needed (the DB CHECK handles enforcement).
func (r *Repository) ValidateBookingForReview(ctx context.Context, bookingID, customerID string) (endTime time.Time, err error) {
	const q = `
		SELECT end_time, status
		FROM bookings
		WHERE id = $1 AND customer_id = $2 AND deleted_at IS NULL`

	var status string
	if err := r.db.QueryRow(ctx, q, bookingID, customerID).Scan(&endTime, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, ErrReviewNotFound
		}
		return time.Time{}, fmt.Errorf("reviews.ValidateBookingForReview: %w", err)
	}
	if status != "completed" {
		return time.Time{}, ErrBookingNotCompleted
	}
	return endTime, nil
}

// GetOwnerIDForService returns the owner.id for the given service (via branch → owner join).
func (r *Repository) GetOwnerIDForService(ctx context.Context, serviceID string) (string, error) {
	const q = `
		SELECT o.id
		FROM services s
		JOIN branches b ON b.id = s.branch_id
		JOIN owners o ON o.id = b.owner_id
		WHERE s.id = $1`

	var ownerID string
	if err := r.db.QueryRow(ctx, q, serviceID).Scan(&ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrReviewNotFound
		}
		return "", fmt.Errorf("reviews.GetOwnerIDForService: %w", err)
	}
	return ownerID, nil
}

// isUniqueViolation returns true when err is a PostgreSQL unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
