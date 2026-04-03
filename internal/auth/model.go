package auth

import "time"

// User is a minimal projection of the users table used by the auth module.
//
// swagger:model AuthUser
type User struct {
	ID            string     `db:"id"`
	Email         string     `db:"email"`
	PasswordHash  *string    `db:"password_hash"`
	Role          string     `db:"role"`
	FullName      *string    `db:"full_name"`
	EmailVerified bool       `db:"email_verified"`
	DeletedAt     *time.Time `db:"deleted_at"`
	CreatedAt     time.Time  `db:"created_at"`
}

// UserIdentity links a user to an SSO provider.
//
// swagger:model UserIdentity
type UserIdentity struct {
	ID         string    `db:"id"`
	UserID     string    `db:"user_id"`
	Provider   string    `db:"provider"`
	ProviderID string    `db:"provider_id"`
	Email      *string   `db:"email"`
	CreatedAt  time.Time `db:"created_at"`
}

// AppleName holds name fields that Apple provides only on first authorization.
//
// swagger:model AppleName
type AppleName struct {
	// example: Juan
	GivenName string `json:"given_name"`
	// example: dela Cruz
	FamilyName string `json:"family_name"`
}

// SSORequest is the request body for POST /auth/sso.
//
// swagger:model SSORequest
type SSORequest struct {
	// SSO provider identifier.
	// required: true
	// enum: google,facebook,apple
	// example: google
	Provider string `json:"provider" binding:"required,oneof=google facebook apple"`

	// Provider-issued identity token.
	// required: true
	// example: eyJhbGciOiJSUzI1NiJ9...
	Token string `json:"token" binding:"required"`

	// Role for new accounts. Defaults to customer if omitted.
	// enum: customer,owner
	// example: customer
	Role string `json:"role"`

	// Apple name — supplied by the mobile app on first Apple sign-in only.
	AppleName *AppleName `json:"apple_name,omitempty"`
}

// SSOResponse is the success response body for POST /auth/sso.
//
// swagger:model SSOResponse
type SSOResponse struct {
	// Signed JWT for the authenticated user.
	// example: eyJhbGciOiJIUzI1NiJ9...
	Token string `json:"token"`
}

// SSOCollisionResponse is returned with HTTP 409 when a new SSO provider's
// email matches an existing account.
//
// swagger:model SSOCollisionResponse
type SSOCollisionResponse struct {
	// Machine-readable code.
	// example: email_exists
	Code string `json:"code"`

	// Short-lived token the client must include in POST /auth/sso/verify-link.
	// example: 3a4b5c6d7e8f9a0b...
	PendingLinkToken string `json:"pending_link_token"`
}

// VerifyLinkRequest is the request body for POST /auth/sso/verify-link.
//
// swagger:model VerifyLinkRequest
type VerifyLinkRequest struct {
	// Pending link token received in the 409 collision response.
	// required: true
	PendingLinkToken string `json:"pending_link_token" binding:"required"`

	// 6-digit OTP sent to the user's email. Mutually exclusive with ProviderToken.
	OTP string `json:"otp"`

	// Re-authentication token from an already-linked provider. Mutually exclusive with OTP.
	ProviderToken string `json:"provider_token"`
}

// SendOTPRequest is the request body for POST /auth/send-otp.
//
// swagger:model SendOTPRequest
type SendOTPRequest struct {
	// Email address to send the OTP to. This endpoint always returns 200.
	// required: true
	// example: juan@example.com
	Email string `json:"email" binding:"required,email"`
}

// VerifyOTPRequest is the request body for POST /auth/verify-otp.
//
// swagger:model VerifyOTPRequest
type VerifyOTPRequest struct {
	// Email the OTP was sent to.
	// required: true
	// example: juan@example.com
	Email string `json:"email" binding:"required,email"`

	// 6-digit OTP.
	// required: true
	// example: 123456
	OTP string `json:"otp" binding:"required"`
}

// SetPasswordRequest is the request body for PUT /auth/password.
//
// swagger:model SetPasswordRequest
type SetPasswordRequest struct {
	// New password. Minimum 8 characters.
	// required: true
	// min length: 8
	// example: P@ssw0rd123
	Password string `json:"password" binding:"required,min=8"`
}

// TokenResponse is a simple JWT response used by verify-otp and verify-link.
//
// swagger:model TokenResponse
type TokenResponse struct {
	// Signed JWT for the authenticated user.
	// example: eyJhbGciOiJIUzI1NiJ9...
	Token string `json:"token"`
}

// --- swagger:response wrappers ---

// swagger:response ssoResponse
type ssoResponseWrapper struct {
	// in: body
	Body SSOResponse
}

// swagger:response ssoCollisionResponse
type ssoCollisionResponseWrapper struct {
	// in: body
	Body SSOCollisionResponse
}

// swagger:response tokenResponse
type tokenResponseWrapper struct {
	// in: body
	Body TokenResponse
}

// swagger:response errorResponse
type errorResponseWrapper struct {
	// in: body
	Body struct {
		// example: invalid_provider_token
		Error string `json:"error"`
		// example: The provider token could not be verified.
		Message string `json:"message"`
	}
}
