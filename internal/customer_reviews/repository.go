package customer_reviews

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the customer_reviews module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new customer review and populates r.ID and r.SubmittedAt via RETURNING.
// Returns ErrAlreadyReviewed if the booking already has a review (unique constraint 23505).
// Returns ErrReviewWindowExpired if the 14-day window has passed (check constraint 23514).
func (r *Repository) Create(ctx context.Context, rev *CustomerReview) error {
	const q = `
		INSERT INTO customer_reviews (booking_id, customer_id, owner_id, branch_id, rating, body)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, submitted_at`

	row := r.db.QueryRow(ctx, q,
		rev.BookingID, rev.CustomerID, rev.OwnerID, rev.BranchID, rev.Rating, rev.Body,
	)
	err := row.Scan(&rev.ID, &rev.SubmittedAt)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return ErrAlreadyReviewed
		}
		if strings.Contains(err.Error(), "23514") {
			return ErrReviewWindowExpired
		}
		return fmt.Errorf("customer_reviews.Create: %w", err)
	}
	return nil
}

// GetByID fetches a customer review by its ID.
// Returns ErrReviewNotFound if no row is found.
func (r *Repository) GetByID(ctx context.Context, id string) (*CustomerReview, error) {
	const q = `
		SELECT id, booking_id, customer_id, owner_id, branch_id, rating, body, status, submitted_at
		FROM customer_reviews
		WHERE id = $1`

	rev := &CustomerReview{}
	err := r.db.QueryRow(ctx, q, id).Scan(
		&rev.ID, &rev.BookingID, &rev.CustomerID, &rev.OwnerID, &rev.BranchID,
		&rev.Rating, &rev.Body, &rev.Status, &rev.SubmittedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("customer_reviews.GetByID: %w", err)
	}
	return rev, nil
}

// GetByCustomer returns up to 50 reviews written about a customer, excluding removed ones, newest first.
func (r *Repository) GetByCustomer(ctx context.Context, customerID string) ([]CustomerReview, error) {
	const q = `
		SELECT id, booking_id, customer_id, owner_id, branch_id, rating, body, status, submitted_at
		FROM customer_reviews
		WHERE customer_id = $1 AND status != 'removed'
		ORDER BY submitted_at DESC
		LIMIT 50`

	rows, err := r.db.Query(ctx, q, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer_reviews.GetByCustomer: %w", err)
	}
	defer rows.Close()

	var reviews []CustomerReview
	for rows.Next() {
		var rev CustomerReview
		if err := rows.Scan(
			&rev.ID, &rev.BookingID, &rev.CustomerID, &rev.OwnerID, &rev.BranchID,
			&rev.Rating, &rev.Body, &rev.Status, &rev.SubmittedAt,
		); err != nil {
			return nil, fmt.Errorf("customer_reviews.GetByCustomer: scan: %w", err)
		}
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

// GetByBooking fetches the review for a specific booking.
// Returns ErrReviewNotFound if no row is found.
func (r *Repository) GetByBooking(ctx context.Context, bookingID string) (*CustomerReview, error) {
	const q = `
		SELECT id, booking_id, customer_id, owner_id, branch_id, rating, body, status, submitted_at
		FROM customer_reviews
		WHERE booking_id = $1`

	rev := &CustomerReview{}
	err := r.db.QueryRow(ctx, q, bookingID).Scan(
		&rev.ID, &rev.BookingID, &rev.CustomerID, &rev.OwnerID, &rev.BranchID,
		&rev.Rating, &rev.Body, &rev.Status, &rev.SubmittedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("customer_reviews.GetByBooking: %w", err)
	}
	return rev, nil
}

// CreateDispute inserts a new dispute and populates d.ID and d.CreatedAt via RETURNING.
// Returns ErrDisputeAlreadyFiled if the customer already disputed this review (unique constraint 23505).
func (r *Repository) CreateDispute(ctx context.Context, d *CustomerReviewDispute) error {
	const q = `
		INSERT INTO customer_review_disputes (customer_review_id, customer_id, reason, details)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	row := r.db.QueryRow(ctx, q, d.CustomerReviewID, d.CustomerID, d.Reason, d.Details)
	err := row.Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return ErrDisputeAlreadyFiled
		}
		return fmt.Errorf("customer_reviews.CreateDispute: %w", err)
	}
	return nil
}

// ValidateBookingForOwnerReview checks that:
//  1. The booking exists and has status='completed'
//  2. The given owner owns the branch associated with the booking
//
// Returns ErrReviewNotFound if the booking does not exist,
// ErrBookingNotCompleted if the booking is not completed,
// ErrUnauthorized if the owner does not own the branch.
func (r *Repository) ValidateBookingForOwnerReview(ctx context.Context, bookingID, ownerID string) error {
	var status, branchID string
	err := r.db.QueryRow(ctx,
		`SELECT status, branch_id FROM bookings WHERE id = $1 AND deleted_at IS NULL`,
		bookingID,
	).Scan(&status, &branchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReviewNotFound
		}
		return fmt.Errorf("customer_reviews.ValidateBookingForOwnerReview: fetch booking: %w", err)
	}

	if status != "completed" {
		return ErrBookingNotCompleted
	}

	var exists bool
	err = r.db.QueryRow(ctx,
		`SELECT COUNT(*) > 0 FROM owners o JOIN branches b ON b.owner_id = o.id WHERE o.id = $1 AND b.id = $2`,
		ownerID, branchID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("customer_reviews.ValidateBookingForOwnerReview: check ownership: %w", err)
	}
	if !exists {
		return ErrUnauthorized
	}

	return nil
}

// GetOwnerIDByUserID looks up the owner record ID for a given user ID.
// Returns ErrUnauthorized if the user is not an owner.
func (r *Repository) GetOwnerIDByUserID(ctx context.Context, userID string) (string, error) {
	var ownerID string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM owners WHERE user_id = $1`,
		userID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUnauthorized
		}
		return "", fmt.Errorf("customer_reviews.GetOwnerIDByUserID: %w", err)
	}
	return ownerID, nil
}

// GetBranchIDByBooking returns the branch_id for a booking.
// Returns ErrReviewNotFound if the booking does not exist.
func (r *Repository) GetBranchIDByBooking(ctx context.Context, bookingID string) (string, error) {
	var branchID string
	err := r.db.QueryRow(ctx,
		`SELECT branch_id FROM bookings WHERE id = $1 AND deleted_at IS NULL`,
		bookingID,
	).Scan(&branchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrReviewNotFound
		}
		return "", fmt.Errorf("customer_reviews.GetBranchIDByBooking: %w", err)
	}
	return branchID, nil
}

// GetCustomerIDByBooking returns the customer_id for a booking.
// Returns ErrReviewNotFound if the booking does not exist.
func (r *Repository) GetCustomerIDByBooking(ctx context.Context, bookingID string) (string, error) {
	var customerID string
	err := r.db.QueryRow(ctx,
		`SELECT customer_id FROM bookings WHERE id = $1 AND deleted_at IS NULL`,
		bookingID,
	).Scan(&customerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrReviewNotFound
		}
		return "", fmt.Errorf("customer_reviews.GetCustomerIDByBooking: %w", err)
	}
	return customerID, nil
}
