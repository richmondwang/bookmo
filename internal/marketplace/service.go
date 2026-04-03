package marketplace

import (
	"context"
	"fmt"
	"time"
)

// Service holds marketplace business logic.
type Service struct {
	repo   *Repository
	client *PayoutClient
}

// NewService creates a Service.
func NewService(repo *Repository, client *PayoutClient) *Service {
	return &Service{repo: repo, client: client}
}

// RegisterPayoutAccount creates a new payout account for an owner.
// Bank accounts require bank_name; GCash/Maya require only the mobile number.
func (s *Service) RegisterPayoutAccount(ctx context.Context, ownerID string, req *CreatePayoutAccountRequest) (*OwnerPayoutAccount, error) {
	if req.AccountType == "bank" && (req.BankName == nil || *req.BankName == "") {
		return nil, fmt.Errorf("marketplace.RegisterPayoutAccount: bank_name is required for bank accounts")
	}
	acc, err := s.repo.CreatePayoutAccount(ctx, ownerID, req)
	if err != nil {
		return nil, fmt.Errorf("marketplace.RegisterPayoutAccount: %w", err)
	}
	return acc, nil
}

// SetDefaultAccount makes the specified account the owner's default payout account.
// Clears any existing default first (one default per owner enforced in service layer).
func (s *Service) SetDefaultAccount(ctx context.Context, ownerID, accountID string) error {
	// Ensure the account belongs to this owner.
	acc, err := s.repo.GetPayoutAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("marketplace.SetDefaultAccount: %w", err)
	}
	if acc.OwnerID != ownerID {
		return ErrUnauthorized
	}
	if err := s.repo.SetDefaultPayoutAccount(ctx, ownerID, accountID); err != nil {
		return fmt.Errorf("marketplace.SetDefaultAccount: %w", err)
	}
	return nil
}

// VerifyAccountOTP verifies a GCash or Maya payout account via OTP.
// Bank accounts are verified manually by admin — this returns ErrOTPNotSupported for them.
// Stub: in production, send OTP to the mobile number and verify the code.
func (s *Service) VerifyAccountOTP(ctx context.Context, accountID, ownerID, _ string) error {
	acc, err := s.repo.GetPayoutAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("marketplace.VerifyAccountOTP: %w", err)
	}
	if acc.OwnerID != ownerID {
		return ErrUnauthorized
	}
	if acc.AccountType == "bank" {
		return ErrOTPNotSupported
	}
	if acc.IsVerified {
		return ErrAccountAlreadyVerified
	}
	// TODO: validate OTP against what was sent to acc.AccountNumber.
	// For MVP: accept any non-empty OTP and mark verified.
	if err := s.repo.VerifyPayoutAccount(ctx, accountID); err != nil {
		return fmt.Errorf("marketplace.VerifyAccountOTP: %w", err)
	}
	return nil
}

// DeletePayoutAccount soft-deletes a payout account owned by the caller.
func (s *Service) DeletePayoutAccount(ctx context.Context, accountID, ownerID string) error {
	acc, err := s.repo.GetPayoutAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("marketplace.DeletePayoutAccount: %w", err)
	}
	if acc.OwnerID != ownerID {
		return ErrUnauthorized
	}
	if err := s.repo.SoftDeletePayoutAccount(ctx, accountID, ownerID); err != nil {
		return fmt.Errorf("marketplace.DeletePayoutAccount: %w", err)
	}
	return nil
}

// GetEarnings returns earnings for an owner, optionally filtered by status.
func (s *Service) GetEarnings(ctx context.Context, ownerID, status string) ([]OwnerEarning, error) {
	earnings, err := s.repo.ListEarnings(ctx, ownerID, status)
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetEarnings: %w", err)
	}
	return earnings, nil
}

// GetPayouts returns payout history for an owner.
func (s *Service) GetPayouts(ctx context.Context, ownerID string) ([]OwnerPayout, error) {
	payouts, err := s.repo.ListPayouts(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetPayouts: %w", err)
	}
	return payouts, nil
}

// UpdatePayoutSchedule sets the payout frequency for an owner.
func (s *Service) UpdatePayoutSchedule(ctx context.Context, ownerID, schedule string) error {
	if err := s.repo.UpdateOwnerPayoutSchedule(ctx, ownerID, schedule); err != nil {
		return fmt.Errorf("marketplace.UpdatePayoutSchedule: %w", err)
	}
	return nil
}

// RaiseDispute creates a booking dispute if the dispute window is still open.
func (s *Service) RaiseDispute(ctx context.Context, bookingID, customerID string, req *RaiseDisputeRequest) (*BookingDispute, error) {
	booking, err := s.repo.GetBookingForDispute(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("marketplace.RaiseDispute: %w", err)
	}

	if booking.CustomerID != customerID {
		return nil, ErrUnauthorized
	}
	if booking.Status != "completed" {
		return nil, fmt.Errorf("marketplace.RaiseDispute: booking is not completed")
	}

	// Check dispute window.
	if booking.CompletedAt == nil {
		return nil, fmt.Errorf("marketplace.RaiseDispute: booking has no completed_at timestamp")
	}
	settings, err := s.repo.GetPlatformSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("marketplace.RaiseDispute: get settings: %w", err)
	}
	window := time.Duration(settings.DisputeWindowHours) * time.Hour
	if time.Since(*booking.CompletedAt) > window {
		return nil, ErrDisputeWindowClosed
	}

	// Check for existing dispute.
	existing, err := s.repo.GetDisputeByBooking(ctx, bookingID)
	if err == nil && existing != nil {
		return nil, ErrAlreadyDisputed
	}

	d := &BookingDispute{
		BookingID:  bookingID,
		CustomerID: customerID,
		Reason:     req.Reason,
		Details:    req.Details,
	}
	if err := s.repo.CreateDispute(ctx, d); err != nil {
		return nil, fmt.Errorf("marketplace.RaiseDispute: %w", err)
	}

	// Mark the corresponding earning as disputed (if it exists).
	_ = s.repo.SetEarningDisputed(ctx, bookingID)

	return d, nil
}

// AdminResolveDispute resolves a dispute as either 'release' (payout proceeds) or
// 'refund' (customer refunded, earning deleted).
func (s *Service) AdminResolveDispute(ctx context.Context, disputeID string, req *ResolveDisputeRequest) error {
	dispute, err := s.repo.GetDisputeByID(ctx, disputeID)
	if err != nil {
		return fmt.Errorf("marketplace.AdminResolveDispute: %w", err)
	}

	var newStatus string
	switch req.Resolution {
	case "release":
		newStatus = "resolved_release"
		// Re-release the earning so the payout scheduler picks it up.
		if err := s.repo.ReleaseEarning(ctx, dispute.BookingID); err != nil {
			// Earning may not exist yet; that's fine.
			_ = err
		}
	case "refund":
		newStatus = "resolved_refund"
		// Delete the earning — owner gets nothing for this booking.
		_ = s.repo.DeleteEarning(ctx, dispute.BookingID)
		// TODO: initiate a PayMongo refund for the customer.
	default:
		return fmt.Errorf("marketplace.AdminResolveDispute: unknown resolution %q", req.Resolution)
	}

	if err := s.repo.ResolveDispute(ctx, disputeID, newStatus, req.AdminNotes); err != nil {
		return fmt.Errorf("marketplace.AdminResolveDispute: %w", err)
	}
	return nil
}

// AdminVerifyPayoutAccount marks a bank account as verified (manual admin flow).
func (s *Service) AdminVerifyPayoutAccount(ctx context.Context, accountID string) error {
	if err := s.repo.VerifyPayoutAccount(ctx, accountID); err != nil {
		return fmt.Errorf("marketplace.AdminVerifyPayoutAccount: %w", err)
	}
	return nil
}
