package bookings

import "time"

// Booking is the core booking entity.
type Booking struct {
	ID                       string     `db:"id"`
	ServiceID                string     `db:"service_id"`
	BranchID                 string     `db:"branch_id"`
	CustomerID               string     `db:"customer_id"`
	StartTime                time.Time  `db:"start_time"`
	EndTime                  time.Time  `db:"end_time"`
	Quantity                 int        `db:"quantity"`
	Status                   string     `db:"status"`
	PaymentMethod            *string    `db:"payment_method"`
	OwnerResponseDeadline    *time.Time `db:"owner_response_deadline"`
	RescheduledFromBookingID *string    `db:"rescheduled_from_booking_id"`
	RescheduleAttemptCount   int        `db:"reschedule_attempt_count"`
	RejectedReason           *string    `db:"rejected_reason"`
	CancelledBy              *string    `db:"cancelled_by"`
	Currency                 string     `db:"currency"`
	DeletedAt                *time.Time `db:"deleted_at"`
	CreatedAt                time.Time  `db:"created_at"`
}

// BookingLock is a short-lived slot reservation created before payment is initiated.
type BookingLock struct {
	ID        string    `db:"id"`
	ServiceID string    `db:"service_id"`
	BranchID  string    `db:"branch_id"`
	StartTime time.Time `db:"start_time"`
	EndTime   time.Time `db:"end_time"`
	Quantity  int       `db:"quantity"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

// RescheduleRequest is a customer proposal to move a confirmed booking.
type RescheduleRequest struct {
	ID           string    `db:"id"`
	BookingID    string    `db:"booking_id"`
	RequestedBy  string    `db:"requested_by"`
	NewStartTime time.Time `db:"new_start_time"`
	NewEndTime   time.Time `db:"new_end_time"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
}

// CustomerTrustData holds pre-aggregated trust signals for a customer.
type CustomerTrustData struct {
	TotalBookings     int     `db:"total_bookings"`
	CompletedBookings int     `db:"completed_bookings"`
	CompletionRate    float64 `db:"completion_rate"`
	AvgOwnerRating    float64 `db:"avg_owner_rating"`
	TotalOwnerReviews int     `db:"total_owner_reviews"`
}

// QueueItem is an owner queue entry with enriched data.
type QueueItem struct {
	Booking
	ServiceName   string             `db:"service_name"`
	BranchName    string             `db:"branch_name"`
	CustomerName  string             `db:"customer_name"`
	CustomerTrust *CustomerTrustData `db:"customer_trust"`
}

// --- Request DTOs ---

// CreateLockRequest is the HTTP request body for POST /bookings/lock.
type CreateLockRequest struct {
	ServiceID string `json:"service_id" binding:"required"`
	BranchID  string `json:"branch_id" binding:"required"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// CreateBookingRequest is the HTTP request body for POST /bookings.
type CreateBookingRequest struct {
	LockID        string  `json:"lock_id" binding:"required"`
	ServiceID     string  `json:"service_id" binding:"required"`
	BranchID      string  `json:"branch_id" binding:"required"`
	StartTime     string  `json:"start_time" binding:"required"`
	EndTime       string  `json:"end_time" binding:"required"`
	Quantity      int     `json:"quantity" binding:"required,min=1"`
	PaymentMethod *string `json:"payment_method"`
}

// RescheduleBookingRequest is the HTTP request body for POST /bookings/:id/reschedule.
type RescheduleBookingRequest struct {
	NewStartTime string `json:"new_start_time" binding:"required"`
	NewEndTime   string `json:"new_end_time" binding:"required"`
}

// CancelRequest is the optional HTTP request body for cancellation.
type CancelRequest struct {
	Reason *string `json:"reason"`
}

// RejectRequest is the HTTP request body for rejecting a booking.
type RejectRequest struct {
	Reason string `json:"reason"`
}

// --- Response DTOs ---

// BookingResponse is the HTTP response DTO for a booking.
type BookingResponse struct {
	ID                       string  `json:"id"`
	ServiceID                string  `json:"service_id"`
	BranchID                 string  `json:"branch_id"`
	CustomerID               string  `json:"customer_id"`
	StartTime                string  `json:"start_time"`
	EndTime                  string  `json:"end_time"`
	Quantity                 int     `json:"quantity"`
	Status                   string  `json:"status"`
	PaymentMethod            *string `json:"payment_method,omitempty"`
	OwnerResponseDeadline    *string `json:"owner_response_deadline,omitempty"`
	RescheduledFromBookingID *string `json:"rescheduled_from_booking_id,omitempty"`
	RescheduleAttemptCount   int     `json:"reschedule_attempt_count"`
	RejectedReason           *string `json:"rejected_reason,omitempty"`
	CancelledBy              *string `json:"cancelled_by,omitempty"`
	Currency                 string  `json:"currency"`
	CreatedAt                string  `json:"created_at"`
}

// QueueItemResponse is the HTTP response DTO for an owner queue item.
type QueueItemResponse struct {
	BookingResponse
	ServiceName   string             `json:"service_name"`
	BranchName    string             `json:"branch_name"`
	CustomerName  string             `json:"customer_name"`
	CustomerTrust *CustomerTrustData `json:"customer_trust,omitempty"`
}

// toResponse converts a Booking to its response DTO.
func toResponse(b *Booking) BookingResponse {
	resp := BookingResponse{
		ID:                       b.ID,
		ServiceID:                b.ServiceID,
		BranchID:                 b.BranchID,
		CustomerID:               b.CustomerID,
		StartTime:                b.StartTime.Format(time.RFC3339),
		EndTime:                  b.EndTime.Format(time.RFC3339),
		Quantity:                 b.Quantity,
		Status:                   b.Status,
		PaymentMethod:            b.PaymentMethod,
		RescheduledFromBookingID: b.RescheduledFromBookingID,
		RescheduleAttemptCount:   b.RescheduleAttemptCount,
		RejectedReason:           b.RejectedReason,
		CancelledBy:              b.CancelledBy,
		Currency:                 b.Currency,
		CreatedAt:                b.CreatedAt.Format(time.RFC3339),
	}
	if b.OwnerResponseDeadline != nil {
		s := b.OwnerResponseDeadline.Format(time.RFC3339)
		resp.OwnerResponseDeadline = &s
	}
	return resp
}
