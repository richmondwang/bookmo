package reviews

import (
	"context"
	"fmt"
	"strings"
)

// Service contains business logic for the reviews module.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Submit validates the booking and creates a new review.
// The 14-day window is enforced by a DB CHECK constraint — a violation is caught here
// and mapped to ErrReviewWindowExpired. Application code does not re-implement the window check.
func (s *Service) Submit(ctx context.Context, req *SubmitReviewRequest, customerID string) (*Review, error) {
	// 1. Validate booking — must be completed and belong to this customer.
	if _, err := s.repo.ValidateBookingForReview(ctx, req.BookingID, customerID); err != nil {
		return nil, fmt.Errorf("reviews.Submit: %w", err)
	}

	// 2. Build the review row.
	rev := &Review{
		BookingID:   req.BookingID,
		ServiceID:   req.ServiceID,
		BranchID:    req.BranchID,
		CustomerID:  customerID,
		Rating:      req.Rating,
		Body:        req.Body,
		IsAnonymous: req.IsAnonymous,
		Status:      "published",
	}

	// 3. Persist. The DB enforces: UNIQUE(booking_id) → ErrAlreadyReviewed,
	//    CHECK(submitted_at <= end_time + 14 days) → caught and mapped to ErrReviewWindowExpired.
	if err := s.repo.Create(ctx, rev); err != nil {
		// Catch the 14-day CHECK constraint violation (SQLSTATE 23514).
		if strings.Contains(err.Error(), "23514") {
			return nil, ErrReviewWindowExpired
		}
		return nil, fmt.Errorf("reviews.Submit: %w", err)
	}

	return rev, nil
}

// GetByService returns all visible reviews for a service as public-safe views.
// Anonymous reviews have CustomerID set to nil in the response.
func (s *Service) GetByService(ctx context.Context, serviceID string) ([]ReviewPublicView, error) {
	reviews, err := s.repo.GetByService(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("reviews.GetByService: %w", err)
	}

	views := make([]ReviewPublicView, 0, len(reviews))
	for i := range reviews {
		rev := &reviews[i]
		view := ReviewPublicView{
			ID:          rev.ID,
			Rating:      rev.Rating,
			Body:        rev.Body,
			IsAnonymous: rev.IsAnonymous,
			Status:      rev.Status,
			SubmittedAt: rev.SubmittedAt,
		}

		// Only expose customer_id when the review is not anonymous.
		if !rev.IsAnonymous {
			cid := rev.CustomerID
			view.CustomerID = &cid
		}

		// Attach owner response, if any.
		resp, err := s.repo.GetResponseByReview(ctx, rev.ID)
		if err != nil {
			return nil, fmt.Errorf("reviews.GetByService: response for %s: %w", rev.ID, err)
		}
		view.Response = resp

		views = append(views, view)
	}
	return views, nil
}

// GetRatingSummary returns the pre-aggregated rating summary for a service.
func (s *Service) GetRatingSummary(ctx context.Context, serviceID string) (*RatingSummary, error) {
	summary, err := s.repo.GetRatingSummary(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("reviews.GetRatingSummary: %w", err)
	}
	return summary, nil
}

// Flag records a flag against a review. If the review accumulates 3 or more flags,
// its status is automatically set to 'flagged'.
func (s *Service) Flag(ctx context.Context, reviewID, reportedBy, reason string) error {
	// 1. Insert the flag (unique per user per review).
	if err := s.repo.CreateFlag(ctx, reviewID, reportedBy, reason); err != nil {
		return fmt.Errorf("reviews.Flag: %w", err)
	}

	// 2. Count total flags.
	count, err := s.repo.GetFlagCount(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("reviews.Flag: count: %w", err)
	}

	// 3. Auto-flag the review when the threshold is reached.
	if count >= 3 {
		if err := s.repo.UpdateReviewStatus(ctx, reviewID, "flagged"); err != nil {
			return fmt.Errorf("reviews.Flag: update status: %w", err)
		}
	}
	return nil
}

// RespondAsOwner lets the owner of the service respond to a review.
// If a response already exists it is updated; otherwise a new one is created.
func (s *Service) RespondAsOwner(ctx context.Context, reviewID, ownerID, body string) error {
	// 1. Load the review to get its service_id.
	rev, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("reviews.RespondAsOwner: %w", err)
	}

	// 2. Verify the caller owns the service the review was written for.
	serviceOwnerID, err := s.repo.GetOwnerIDForService(ctx, rev.ServiceID)
	if err != nil {
		return fmt.Errorf("reviews.RespondAsOwner: %w", err)
	}
	if serviceOwnerID != ownerID {
		return ErrNotReviewOwner
	}

	// 3. Upsert: try to create first; fall back to update if one already exists.
	resp := &ReviewResponse{
		ReviewID: reviewID,
		OwnerID:  ownerID,
		Body:     body,
	}
	if err := s.repo.CreateResponse(ctx, resp); err != nil {
		// If a response already exists, update it.
		if strings.Contains(err.Error(), "response already exists") {
			if err := s.repo.UpdateResponse(ctx, reviewID, body); err != nil {
				return fmt.Errorf("reviews.RespondAsOwner: update: %w", err)
			}
			return nil
		}
		return fmt.Errorf("reviews.RespondAsOwner: create: %w", err)
	}
	return nil
}
