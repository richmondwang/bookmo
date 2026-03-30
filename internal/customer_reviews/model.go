package customer_reviews

import "time"

// CustomerReview is the owner-written review of a customer after a completed booking.
type CustomerReview struct {
	ID          string    `json:"id"`
	BookingID   string    `json:"booking_id"`
	CustomerID  string    `json:"customer_id"`
	OwnerID     string    `json:"owner_id"`
	BranchID    string    `json:"branch_id"`
	Rating      int       `json:"rating"`
	Body        *string   `json:"body,omitempty"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// CustomerReviewDispute is a customer's formal contest of an owner-written review.
type CustomerReviewDispute struct {
	ID               string    `json:"id"`
	CustomerReviewID string    `json:"customer_review_id"`
	CustomerID       string    `json:"customer_id"`
	Reason           string    `json:"reason"`
	Details          *string   `json:"details,omitempty"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

// SubmitReviewRequest is the HTTP request body for POST /customer-reviews.
type SubmitReviewRequest struct {
	BookingID string  `json:"booking_id" binding:"required"`
	Rating    int     `json:"rating"     binding:"required,min=1,max=5"`
	Body      *string `json:"body"`
}

// DisputeRequest is the HTTP request body for POST /customer-reviews/:id/dispute.
type DisputeRequest struct {
	Reason  string  `json:"reason"  binding:"required,oneof=inaccurate inappropriate not_my_booking retaliation"`
	Details *string `json:"details"`
}
