package customer_reviews

import (
	"context"
	"fmt"
)

// Service contains the business logic for the customer_reviews module.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Submit allows an owner to write a review of a customer after a completed booking.
// The 14-day window is enforced by a DB CHECK constraint — not duplicated here.
func (s *Service) Submit(ctx context.Context, req *SubmitReviewRequest, ownerUserID string) (*CustomerReview, error) {
	// 1. Look up the owner record for this user.
	ownerID, err := s.repo.GetOwnerIDByUserID(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("customer_reviews.Submit: %w", err)
	}

	// 2. Validate the booking is completed and the owner owns the branch.
	if err := s.repo.ValidateBookingForOwnerReview(ctx, req.BookingID, ownerID); err != nil {
		return nil, fmt.Errorf("customer_reviews.Submit: %w", err)
	}

	// 3. Get the branch ID for the review record.
	branchID, err := s.repo.GetBranchIDByBooking(ctx, req.BookingID)
	if err != nil {
		return nil, fmt.Errorf("customer_reviews.Submit: %w", err)
	}

	// 4. Get the customer ID for the review record.
	customerID, err := s.repo.GetCustomerIDByBooking(ctx, req.BookingID)
	if err != nil {
		return nil, fmt.Errorf("customer_reviews.Submit: %w", err)
	}

	// 5. Create the review. The DB CHECK constraint enforces the 14-day window.
	//    ErrAlreadyReviewed and ErrReviewWindowExpired are surfaced directly from repo.Create.
	rev := &CustomerReview{
		BookingID:  req.BookingID,
		CustomerID: customerID,
		OwnerID:    ownerID,
		BranchID:   branchID,
		Rating:     req.Rating,
		Body:       req.Body,
		Status:     "published",
	}
	if err := s.repo.Create(ctx, rev); err != nil {
		return nil, fmt.Errorf("customer_reviews.Submit: %w", err)
	}

	return rev, nil
}

// GetByCustomer returns reviews written about a customer.
// The caller must enforce visibility rules: owners can call for any customer;
// customers can only call for themselves.
func (s *Service) GetByCustomer(ctx context.Context, customerID string) ([]CustomerReview, error) {
	reviews, err := s.repo.GetByCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer_reviews.GetByCustomer: %w", err)
	}
	return reviews, nil
}

// Dispute allows the reviewed customer to file a dispute against a review.
// Per ADR-007, disputes do NOT hide reviews — the review stays published until
// an admin explicitly removes it.
func (s *Service) Dispute(ctx context.Context, reviewID, customerUserID string, req *DisputeRequest) error {
	// 1. Fetch the review to verify ownership.
	rev, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("customer_reviews.Dispute: %w", err)
	}

	// 2. Only the reviewed customer can file a dispute.
	if rev.CustomerID != customerUserID {
		return ErrNotYourReview
	}

	// 3. Create the dispute. ErrDisputeAlreadyFiled is surfaced from repo.CreateDispute.
	d := &CustomerReviewDispute{
		CustomerReviewID: reviewID,
		CustomerID:       customerUserID,
		Reason:           req.Reason,
		Details:          req.Details,
		Status:           "open",
	}
	if err := s.repo.CreateDispute(ctx, d); err != nil {
		return fmt.Errorf("customer_reviews.Dispute: %w", err)
	}

	// Review intentionally stays published — no status update here (ADR-007).
	return nil
}
