package customer_reviews

import "errors"

var (
	ErrAlreadyReviewed     = errors.New("customer_reviews: booking already reviewed")
	ErrBookingNotCompleted = errors.New("customer_reviews: booking not completed")
	ErrReviewWindowExpired = errors.New("customer_reviews: review window expired")
	ErrDisputeAlreadyFiled = errors.New("customer_reviews: dispute already filed")
	ErrNotYourReview       = errors.New("customer_reviews: not your review")
	ErrReviewNotFound      = errors.New("customer_reviews: not found")
	ErrUnauthorized        = errors.New("customer_reviews: unauthorized")
)
