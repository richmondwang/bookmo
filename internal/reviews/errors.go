package reviews

import "errors"

var (
	ErrReviewNotFound      = errors.New("reviews: not found")
	ErrAlreadyReviewed     = errors.New("reviews: already reviewed")
	ErrBookingNotCompleted = errors.New("reviews: booking not completed")
	ErrReviewWindowExpired = errors.New("reviews: review window expired")
	ErrNotBookingOwner     = errors.New("reviews: not the booking owner")
	ErrNotReviewOwner      = errors.New("reviews: not the review owner")
	ErrAlreadyFlagged      = errors.New("reviews: already flagged by this user")
)
