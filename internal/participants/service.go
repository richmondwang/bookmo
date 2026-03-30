package participants

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const notificationQueue = "notification_jobs"

// Service orchestrates participant business logic.
type Service struct {
	repo *Repository
	rdb  *redis.Client
}

// NewService constructs a Service.
func NewService(repo *Repository, rdb *redis.Client) *Service {
	return &Service{repo: repo, rdb: rdb}
}

// enqueueNotification pushes a push-notification job onto the Redis queue.
// Failure is non-fatal — the participant state is already persisted.
func (s *Service) enqueueNotification(ctx context.Context, userID, notifType, title, body string) {
	job := map[string]any{
		"user_id": userID,
		"type":    notifType,
		"title":   title,
		"body":    body,
	}
	data, _ := json.Marshal(job)
	_ = s.rdb.LPush(ctx, notificationQueue, data).Err()
}

// Invite tags a user on a booking. Validation order follows ADR-008 and ADR-009:
//  1. Eligibility check (service/category chain) — ErrParticipantsNotAllowed
//  2. Booking must exist and caller must be the creator — ErrNotBookingCreator
//  3. Booking must not be completed — ErrBookingCompleted
//  4. Cannot invite yourself — ErrCannotInviteSelf
//  5. Insert (UNIQUE constraint) — ErrAlreadyInvited
func (s *Service) Invite(ctx context.Context, bookingID, callerID, targetUserID string) (*BookingParticipant, error) {
	eligible, err := s.repo.ResolveParticipantEligibility(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("participants.Invite: %w", err)
	}
	if !eligible {
		return nil, ErrParticipantsNotAllowed
	}

	customerID, status, err := s.repo.GetBookingForParticipant(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("participants.Invite: %w", err)
	}

	if customerID != callerID {
		return nil, ErrNotBookingCreator
	}
	if status == "completed" {
		return nil, ErrBookingCompleted
	}
	if targetUserID == customerID {
		return nil, ErrCannotInviteSelf
	}

	p := &BookingParticipant{
		BookingID: bookingID,
		UserID:    targetUserID,
		InvitedBy: callerID,
		Status:    "pending",
	}
	if err := s.repo.Invite(ctx, p); err != nil {
		return nil, fmt.Errorf("participants.Invite: %w", err)
	}

	s.enqueueNotification(ctx, targetUserID, "participant_invited",
		"You've been invited to a booking",
		"Someone has added you to their booking. Tap to view and respond.")

	return p, nil
}

// GetByBooking returns all participants for the given booking.
func (s *Service) GetByBooking(ctx context.Context, bookingID string) ([]BookingParticipant, error) {
	participants, err := s.repo.GetByBooking(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("participants.GetByBooking: %w", err)
	}
	return participants, nil
}

// Accept marks the caller as an accepted participant and notifies the booking creator.
func (s *Service) Accept(ctx context.Context, bookingID, callerID string) error {
	p, err := s.repo.GetParticipant(ctx, bookingID, callerID)
	if err != nil {
		if errors.Is(err, ErrParticipantNotFound) {
			return ErrNotParticipant
		}
		return fmt.Errorf("participants.Accept: %w", err)
	}
	if p.UserID != callerID {
		return ErrNotParticipant
	}

	customerID, status, err := s.repo.GetBookingForParticipant(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("participants.Accept: %w", err)
	}
	if status == "completed" {
		return ErrBookingCompleted
	}

	if err := s.repo.Accept(ctx, bookingID, callerID); err != nil {
		return fmt.Errorf("participants.Accept: %w", err)
	}

	s.enqueueNotification(ctx, customerID, "participant_accepted",
		"Someone accepted your booking invite",
		"A participant accepted your invitation to join your booking.")

	return nil
}

// Decline marks the caller as a declined participant.
func (s *Service) Decline(ctx context.Context, bookingID, callerID string) error {
	p, err := s.repo.GetParticipant(ctx, bookingID, callerID)
	if err != nil {
		if errors.Is(err, ErrParticipantNotFound) {
			return ErrNotParticipant
		}
		return fmt.Errorf("participants.Decline: %w", err)
	}
	if p.UserID != callerID {
		return ErrNotParticipant
	}

	if err := s.repo.Decline(ctx, bookingID, callerID); err != nil {
		return fmt.Errorf("participants.Decline: %w", err)
	}
	return nil
}

// Leave removes the caller from the booking. Not allowed once the booking is completed.
func (s *Service) Leave(ctx context.Context, bookingID, callerID string) error {
	p, err := s.repo.GetParticipant(ctx, bookingID, callerID)
	if err != nil {
		if errors.Is(err, ErrParticipantNotFound) {
			return ErrNotParticipant
		}
		return fmt.Errorf("participants.Leave: %w", err)
	}
	if p.UserID != callerID {
		return ErrNotParticipant
	}

	_, status, err := s.repo.GetBookingForParticipant(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("participants.Leave: %w", err)
	}
	if status == "completed" {
		return ErrBookingCompleted
	}

	if err := s.repo.Leave(ctx, bookingID, callerID); err != nil {
		return fmt.Errorf("participants.Leave: %w", err)
	}
	return nil
}

// GetBookedWith returns users who have completed bookings with the given user.
func (s *Service) GetBookedWith(ctx context.Context, userID string) ([]BookedWithUser, error) {
	users, err := s.repo.GetBookedWith(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("participants.GetBookedWith: %w", err)
	}
	return users, nil
}
