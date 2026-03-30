package bookings

import (
	"context"
	"fmt"
	"time"
)

const (
	maxRescheduleAttempts = 3
	lockTTL               = 5 * time.Minute
	ownerResponseWindow   = 24 * time.Hour
)

// Service contains the business logic for the bookings module.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// LockSlot validates times and creates a short-lived booking lock (5-minute TTL).
func (s *Service) LockSlot(ctx context.Context, req *CreateLockRequest) (*BookingLock, error) {
	start, end, err := parseTimes(req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("bookings.LockSlot: %w", err)
	}

	conflict, err := s.repo.CheckSlotConflict(ctx, req.ServiceID, start, end, req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("bookings.LockSlot: %w", err)
	}
	if conflict {
		return nil, ErrSlotUnavailable
	}

	lock := &BookingLock{
		ServiceID: req.ServiceID,
		BranchID:  req.BranchID,
		StartTime: start,
		EndTime:   end,
		Quantity:  req.Quantity,
		ExpiresAt: time.Now().UTC().Add(lockTTL),
	}
	if err := s.repo.CreateLock(ctx, lock); err != nil {
		return nil, fmt.Errorf("bookings.LockSlot: %w", err)
	}
	return lock, nil
}

// CreateBooking validates the lock and creates a booking with status 'pending'.
func (s *Service) CreateBooking(ctx context.Context, req *CreateBookingRequest, customerID string) (*Booking, error) {
	lock, err := s.repo.GetLock(ctx, req.LockID)
	if err != nil {
		return nil, fmt.Errorf("bookings.CreateBooking: %w", err)
	}

	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return nil, fmt.Errorf("bookings.CreateBooking: invalid start_time: %w", err)
	}
	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("bookings.CreateBooking: invalid end_time: %w", err)
	}

	b := &Booking{
		ServiceID:     lock.ServiceID,
		BranchID:      lock.BranchID,
		CustomerID:    customerID,
		StartTime:     start,
		EndTime:       end,
		Quantity:      req.Quantity,
		Status:        "pending",
		PaymentMethod: req.PaymentMethod,
		Currency:      "PHP",
	}

	if err := s.repo.Create(ctx, b); err != nil {
		return nil, fmt.Errorf("bookings.CreateBooking: %w", err)
	}

	// Delete lock now that the booking exists.
	if err := s.repo.DeleteLock(ctx, req.LockID); err != nil {
		// Non-fatal: lock will expire naturally. Log but don't fail.
		_ = err
	}

	return b, nil
}

// ConfirmPayment transitions a booking from pending → awaiting_approval and sets owner_response_deadline.
func (s *Service) ConfirmPayment(ctx context.Context, bookingID string) error {
	b, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("bookings.ConfirmPayment: %w", err)
	}
	if b.Status != "pending" {
		return fmt.Errorf("bookings.ConfirmPayment: %w", ErrIllegalStateTransition)
	}

	deadline := time.Now().UTC().Add(ownerResponseWindow)
	if err := s.repo.UpdateStatus(ctx, bookingID, "awaiting_approval", map[string]any{
		"owner_response_deadline": deadline,
	}); err != nil {
		return fmt.Errorf("bookings.ConfirmPayment: %w", err)
	}
	return nil
}

// Approve transitions a booking from awaiting_approval → confirmed, after validating ownership and re-checking the slot.
func (s *Service) Approve(ctx context.Context, bookingID, ownerID string) error {
	owns, err := s.repo.OwnsBookingBranch(ctx, bookingID, ownerID)
	if err != nil {
		return fmt.Errorf("bookings.Approve: %w", err)
	}
	if !owns {
		return ErrUnauthorized
	}

	b, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("bookings.Approve: %w", err)
	}
	if b.Status != "awaiting_approval" {
		return fmt.Errorf("bookings.Approve: %w", ErrIllegalStateTransition)
	}

	// Re-check slot availability (another booking may have landed during the 24h window).
	conflict, err := s.repo.CheckSlotConflict(ctx, b.ServiceID, b.StartTime, b.EndTime, b.Quantity)
	if err != nil {
		return fmt.Errorf("bookings.Approve: %w", err)
	}
	if conflict {
		return ErrSlotUnavailable
	}

	if err := s.repo.UpdateStatus(ctx, bookingID, "confirmed", nil); err != nil {
		return fmt.Errorf("bookings.Approve: %w", err)
	}
	return nil
}

// Reject transitions a booking from awaiting_approval → rejected.
func (s *Service) Reject(ctx context.Context, bookingID, ownerID, reason string) error {
	owns, err := s.repo.OwnsBookingBranch(ctx, bookingID, ownerID)
	if err != nil {
		return fmt.Errorf("bookings.Reject: %w", err)
	}
	if !owns {
		return ErrUnauthorized
	}

	b, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("bookings.Reject: %w", err)
	}
	if b.Status != "awaiting_approval" {
		return fmt.Errorf("bookings.Reject: %w", ErrIllegalStateTransition)
	}

	extra := map[string]any{}
	if reason != "" {
		extra["rejected_reason"] = reason
	}
	if err := s.repo.UpdateStatus(ctx, bookingID, "rejected", extra); err != nil {
		return fmt.Errorf("bookings.Reject: %w", err)
	}
	return nil
}

// Cancel transitions a booking from confirmed → cancelled.
// callerRole must be "customer", "owner", or "system".
func (s *Service) Cancel(ctx context.Context, bookingID, callerID, callerRole string) error {
	b, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("bookings.Cancel: %w", err)
	}
	if b.Status != "confirmed" {
		return fmt.Errorf("bookings.Cancel: %w", ErrIllegalStateTransition)
	}

	// Authorization: customer can only cancel their own booking.
	if callerRole == "customer" && b.CustomerID != callerID {
		return ErrUnauthorized
	}
	// Authorization: owner must own the branch.
	if callerRole == "owner" {
		owns, err := s.repo.OwnsBookingBranch(ctx, bookingID, callerID)
		if err != nil {
			return fmt.Errorf("bookings.Cancel: %w", err)
		}
		if !owns {
			return ErrUnauthorized
		}
	}

	if err := s.repo.UpdateStatus(ctx, bookingID, "cancelled", map[string]any{
		"cancelled_by": callerRole,
	}); err != nil {
		return fmt.Errorf("bookings.Cancel: %w", err)
	}
	return nil
}

// RequestReschedule creates a reschedule request for a confirmed booking.
func (s *Service) RequestReschedule(ctx context.Context, bookingID, customerID string, req *RescheduleBookingRequest) error {
	b, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("bookings.RequestReschedule: %w", err)
	}
	if b.Status != "confirmed" {
		return fmt.Errorf("bookings.RequestReschedule: %w", ErrIllegalStateTransition)
	}
	if b.CustomerID != customerID {
		return ErrUnauthorized
	}
	if b.RescheduleAttemptCount >= maxRescheduleAttempts {
		return ErrRescheduleLimitReached
	}

	// Ensure no existing pending reschedule.
	existing, err := s.repo.GetPendingRescheduleForBooking(ctx, bookingID)
	if err != nil && err != ErrRescheduleNotFound {
		return fmt.Errorf("bookings.RequestReschedule: %w", err)
	}
	if existing != nil {
		return ErrPendingRescheduleExists
	}

	newStart, newEnd, err := parseTimes(req.NewStartTime, req.NewEndTime)
	if err != nil {
		return fmt.Errorf("bookings.RequestReschedule: %w", err)
	}

	rr := &RescheduleRequest{
		BookingID:    bookingID,
		RequestedBy:  customerID,
		NewStartTime: newStart,
		NewEndTime:   newEnd,
		Status:       "pending",
	}
	if err := s.repo.CreateRescheduleRequest(ctx, rr); err != nil {
		return fmt.Errorf("bookings.RequestReschedule: %w", err)
	}

	// Increment attempt count on the booking.
	if err := s.repo.UpdateStatus(ctx, bookingID, b.Status, map[string]any{
		"reschedule_attempt_count": b.RescheduleAttemptCount + 1,
	}); err != nil {
		return fmt.Errorf("bookings.RequestReschedule: increment attempts: %w", err)
	}

	return nil
}

// ApproveReschedule approves a pending reschedule, creating a new confirmed booking.
func (s *Service) ApproveReschedule(ctx context.Context, rescheduleID, ownerID string) error {
	owns, err := s.repo.OwnsRescheduleBranch(ctx, rescheduleID, ownerID)
	if err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: %w", err)
	}
	if !owns {
		return ErrUnauthorized
	}

	rr, err := s.repo.GetRescheduleByID(ctx, rescheduleID)
	if err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: %w", err)
	}

	// Load original booking to get customer_id and payment method for the new booking.
	orig, err := s.repo.GetByID(ctx, rr.BookingID)
	if err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: %w", err)
	}

	newBooking := &Booking{
		CustomerID:    orig.CustomerID,
		PaymentMethod: orig.PaymentMethod,
		Currency:      orig.Currency,
	}

	if err := s.repo.ApproveReschedule(ctx, rescheduleID, newBooking); err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: %w", err)
	}
	return nil
}

// RejectReschedule rejects a pending reschedule request.
func (s *Service) RejectReschedule(ctx context.Context, rescheduleID, ownerID string) error {
	owns, err := s.repo.OwnsRescheduleBranch(ctx, rescheduleID, ownerID)
	if err != nil {
		return fmt.Errorf("bookings.RejectReschedule: %w", err)
	}
	if !owns {
		return ErrUnauthorized
	}

	rr, err := s.repo.GetRescheduleByID(ctx, rescheduleID)
	if err != nil {
		return fmt.Errorf("bookings.RejectReschedule: %w", err)
	}
	if rr.Status != "pending" {
		return fmt.Errorf("bookings.RejectReschedule: %w", ErrIllegalStateTransition)
	}

	if err := s.repo.UpdateRescheduleStatus(ctx, rescheduleID, "rejected"); err != nil {
		return fmt.Errorf("bookings.RejectReschedule: %w", err)
	}
	return nil
}

// GetOwnerQueue returns the owner's pending approval queue.
func (s *Service) GetOwnerQueue(ctx context.Context, ownerID string) ([]QueueItem, error) {
	items, err := s.repo.GetOwnerQueue(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("bookings.GetOwnerQueue: %w", err)
	}
	return items, nil
}

// GetCustomerBookings returns all bookings for a customer.
func (s *Service) GetCustomerBookings(ctx context.Context, customerID string) ([]Booking, error) {
	bs, err := s.repo.GetBookingsByCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("bookings.GetCustomerBookings: %w", err)
	}
	return bs, nil
}

// parseTimes parses two RFC3339 time strings and validates that start is before end.
func parseTimes(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time: %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time: %w", err)
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_time must be before end_time")
	}
	return start.UTC(), end.UTC(), nil
}
