package auth

import "errors"

var (
	// ErrNotFound is returned when a user or identity record does not exist.
	ErrNotFound = errors.New("auth: not found")
	// ErrInvalidProviderToken is returned when SSO provider token verification fails.
	ErrInvalidProviderToken = errors.New("auth: invalid provider token")
	// ErrInvalidOTP is returned when the supplied OTP does not match.
	ErrInvalidOTP = errors.New("auth: invalid OTP")
	// ErrOTPMaxAttempts is returned when the OTP attempt limit is exceeded.
	ErrOTPMaxAttempts = errors.New("auth: OTP max attempts reached")
	// ErrInvalidCredentials is returned for bad email/password login.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
)

// ErrEmailCollision is returned (HTTP 409) when a new SSO provider's email
// matches an existing account. The caller must complete the verify-link flow
// before the new identity is linked.
type ErrEmailCollision struct {
	PendingLinkToken string
}

func (e *ErrEmailCollision) Error() string {
	return "auth: email already exists — verification required before linking"
}
