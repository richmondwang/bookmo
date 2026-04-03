package marketplace

import "errors"

var (
	// ErrNoVerifiedPayoutAccount is returned when a payout is initiated but the
	// owner has no verified default payout account.
	ErrNoVerifiedPayoutAccount = errors.New("marketplace: no verified default payout account")

	// ErrDisputeWindowClosed is returned when a dispute is raised after the
	// dispute window has expired.
	ErrDisputeWindowClosed = errors.New("marketplace: dispute window has closed")

	// ErrAlreadyDisputed is returned when a dispute already exists for a booking.
	ErrAlreadyDisputed = errors.New("marketplace: booking already disputed")

	// ErrInvalidFeeConfig is returned when the fee resolution chain has no valid entry.
	ErrInvalidFeeConfig = errors.New("marketplace: no fee configuration found")

	// ErrPayoutFailed is returned when the PayMongo payout transfer fails.
	ErrPayoutFailed = errors.New("marketplace: payout transfer failed")

	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("marketplace: not found")

	// ErrUnauthorized is returned when the caller does not own the resource.
	ErrUnauthorized = errors.New("marketplace: unauthorized")

	// ErrAccountAlreadyVerified is returned when verifying an already-verified account.
	ErrAccountAlreadyVerified = errors.New("marketplace: payout account already verified")

	// ErrOTPNotSupported is returned when OTP verification is requested for a bank account.
	ErrOTPNotSupported = errors.New("marketplace: OTP verification not supported for bank accounts")
)
