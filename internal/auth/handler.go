package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the auth module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all auth routes on the given router group.
// Public routes (no JWT) are registered directly; the authenticated PUT /auth/password
// route requires the caller to add RequireAuth middleware to the group or a sub-group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/sso", h.ssoLogin)
	rg.POST("/auth/sso/verify-link", h.verifyLink)
	rg.POST("/auth/send-otp", h.sendOTP)
	rg.POST("/auth/verify-otp", h.verifyOTP)
	rg.PUT("/auth/password", h.setPassword) // caller must add RequireAuth middleware
}

// ssoLogin handles POST /auth/sso.
//
// swagger:route POST /auth/sso auth ssoLogin
//
// # Authenticate via SSO provider
//
// Verifies a provider-issued identity token (Google, Facebook, or Apple) and
// returns a signed JWT. If the provider's email matches an existing account from a
// different provider, returns HTTP 409 with a pending_link_token that must be
// used to complete account linking via POST /auth/sso/verify-link.
//
// Responses:
//
//	200: ssoResponse
//	409: ssoCollisionResponse
//	400: errorResponse
//	401: errorResponse
func (h *Handler) ssoLogin(c *gin.Context) {
	var req SSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	token, err := h.svc.AuthenticateSSO(c.Request.Context(), &req)
	if err != nil {
		var collision *ErrEmailCollision
		if errors.As(err, &collision) {
			c.JSON(http.StatusConflict, SSOCollisionResponse{
				Code:             "email_exists",
				PendingLinkToken: collision.PendingLinkToken,
			})
			return
		}
		if errors.Is(err, ErrInvalidProviderToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_provider_token", "message": "The provider token could not be verified"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}

	c.JSON(http.StatusOK, SSOResponse{Token: token})
}

// verifyLink handles POST /auth/sso/verify-link.
//
// swagger:route POST /auth/sso/verify-link auth ssoVerifyLink
//
// # Complete SSO account link after email collision
//
// Verifies identity using a 6-digit OTP (or re-authentication token) and links
// the new SSO provider to the existing account. Returns a signed JWT on success.
//
// Responses:
//
//	200: tokenResponse
//	400: errorResponse
//	401: errorResponse
//	404: errorResponse
func (h *Handler) verifyLink(c *gin.Context) {
	var req VerifyLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	token, err := h.svc.VerifyPendingLink(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Pending link token expired or not found"})
		case errors.Is(err, ErrInvalidOTP):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_otp", "message": "The OTP is incorrect"})
		case errors.Is(err, ErrOTPMaxAttempts):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "otp_max_attempts", "message": "Too many incorrect attempts. Please request a new OTP."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		}
		return
	}

	c.JSON(http.StatusOK, TokenResponse{Token: token})
}

// sendOTP handles POST /auth/send-otp.
//
// swagger:route POST /auth/send-otp auth sendOTP
//
// # Send OTP to email
//
// Sends a 6-digit one-time password to the provided email address.
// Always returns 200 regardless of whether the email exists to prevent
// account enumeration. The OTP expires after 10 minutes and allows
// a maximum of 3 verification attempts.
//
// Responses:
//
//	200: {}
func (h *Handler) sendOTP(c *gin.Context) {
	var req SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Still return 200 to avoid leaking whether the email exists.
		c.Status(http.StatusOK)
		return
	}

	// Errors are swallowed — always return 200.
	_ = h.svc.SendOTP(c.Request.Context(), req.Email)
	c.Status(http.StatusOK)
}

// verifyOTP handles POST /auth/verify-otp.
//
// swagger:route POST /auth/verify-otp auth verifyOTP
//
// # Verify OTP and issue JWT
//
// Verifies a 6-digit OTP for the given email and returns a signed JWT.
// Used in the email-verification flow for email/password users.
//
// Responses:
//
//	200: tokenResponse
//	400: errorResponse
//	401: errorResponse
func (h *Handler) verifyOTP(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	token, err := h.svc.VerifyOTP(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidOTP), errors.Is(err, ErrOTPMaxAttempts):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_otp", "message": "The OTP is incorrect or has expired"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		}
		return
	}

	c.JSON(http.StatusOK, TokenResponse{Token: token})
}

// setPassword handles PUT /auth/password.
//
// swagger:route PUT /auth/password auth setPassword
//
// # Set or update account password
//
// Allows an authenticated user (including SSO-only users) to set or update
// their account password. Once set, both SSO and email/password authentication
// work simultaneously.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	204: {}
//	400: errorResponse
//	401: errorResponse
func (h *Handler) setPassword(c *gin.Context) {
	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		return
	}

	if err := h.svc.SetPassword(c.Request.Context(), userID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}

	c.Status(http.StatusNoContent)
}
