package reviews

import "time"

// Review is the internal DB model for a customer review.
type Review struct {
	ID          string    `json:"id"`
	BookingID   string    `json:"-"` // never expose in API
	ServiceID   string    `json:"service_id"`
	BranchID    string    `json:"branch_id"`
	CustomerID  string    `json:"-"` // conditionally exposed (not if anonymous)
	Rating      int       `json:"rating"`
	Body        *string   `json:"body,omitempty"`
	IsAnonymous bool      `json:"is_anonymous"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// ReviewResponse is the owner's public reply to a review.
type ReviewResponse struct {
	ID        string     `json:"id"`
	ReviewID  string     `json:"review_id"`
	OwnerID   string     `json:"owner_id"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// RatingSummary is the pre-aggregated rating row for a service.
// It is written by a DB trigger — never written directly by application code.
type RatingSummary struct {
	ServiceID    string  `json:"service_id"`
	TotalReviews int     `json:"total_reviews"`
	AvgRating    float64 `json:"avg_rating"`
	FiveStar     int     `json:"five_star"`
	FourStar     int     `json:"four_star"`
	ThreeStar    int     `json:"three_star"`
	TwoStar      int     `json:"two_star"`
	OneStar      int     `json:"one_star"`
}

// ReviewPublicView is the API response shape for a review.
// CustomerID is nil when the review is anonymous.
type ReviewPublicView struct {
	ID          string          `json:"id"`
	Rating      int             `json:"rating"`
	Body        *string         `json:"body,omitempty"`
	IsAnonymous bool            `json:"is_anonymous"`
	CustomerID  *string         `json:"customer_id,omitempty"` // nil when anonymous
	Status      string          `json:"status"`
	SubmittedAt time.Time       `json:"submitted_at"`
	Response    *ReviewResponse `json:"response,omitempty"`
}

// SubmitReviewRequest is the HTTP request body for submitting a review.
type SubmitReviewRequest struct {
	BookingID   string  `json:"booking_id" binding:"required"`
	ServiceID   string  `json:"service_id" binding:"required"`
	BranchID    string  `json:"branch_id" binding:"required"`
	Rating      int     `json:"rating" binding:"required,min=1,max=5"`
	Body        *string `json:"body"`
	IsAnonymous bool    `json:"is_anonymous"`
}

// FlagReviewRequest is the HTTP request body for flagging a review.
type FlagReviewRequest struct {
	Reason string `json:"reason" binding:"required,oneof=spam offensive fake irrelevant"`
}

// ReviewResponseRequest is the HTTP request body for an owner response.
type ReviewResponseRequest struct {
	Body string `json:"body" binding:"required,max=500"`
}
