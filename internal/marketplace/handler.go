package marketplace

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves owner-facing marketplace HTTP routes.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterOwnerRoutes registers owner-facing routes on the given group.
// The caller must apply RequireAuth and RequireRole("owner") middleware.
func (h *Handler) RegisterOwnerRoutes(rg *gin.RouterGroup) {
	owner := rg.Group("/owner")
	owner.GET("/payout-accounts", h.listPayoutAccounts)
	owner.POST("/payout-accounts", h.createPayoutAccount)
	owner.PUT("/payout-accounts/:id", h.setDefaultAccount)
	owner.DELETE("/payout-accounts/:id", h.deletePayoutAccount)
	owner.POST("/payout-accounts/:id/verify-otp", h.verifyAccountOTP)
	owner.GET("/earnings", h.listEarnings)
	owner.GET("/payouts", h.listPayouts)
	owner.PUT("/payout-schedule", h.updatePayoutSchedule)
}

// listPayoutAccounts handles GET /owner/payout-accounts.
//
// swagger:route GET /owner/payout-accounts owner listPayoutAccounts
//
// # List owner payout accounts
//
// Returns all active payout accounts registered by the authenticated owner.
// Requires role: owner.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: payoutAccountListResponse
//	401: errorResponse
//	403: errorResponse
func (h *Handler) listPayoutAccounts(c *gin.Context) {
	ownerID := c.GetString("user_id")
	accounts, err := h.svc.repo.ListPayoutAccounts(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if accounts == nil {
		accounts = []OwnerPayoutAccount{}
	}
	c.JSON(http.StatusOK, accounts)
}

// createPayoutAccount handles POST /owner/payout-accounts.
//
// swagger:route POST /owner/payout-accounts owner createPayoutAccount
//
// # Register a payout account
//
// Registers a new GCash, Maya, or bank account to receive payouts.
// GCash and Maya accounts must be verified via OTP before receiving payouts.
// Bank accounts require manual admin verification. Requires role: owner.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	201: payoutAccountResponse
//	400: errorResponse
//	401: errorResponse
//	403: errorResponse
func (h *Handler) createPayoutAccount(c *gin.Context) {
	var req CreatePayoutAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	ownerID := c.GetString("user_id")
	acc, err := h.svc.RegisterPayoutAccount(c.Request.Context(), ownerID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, acc)
}

// setDefaultAccount handles PUT /owner/payout-accounts/:id.
//
// swagger:route PUT /owner/payout-accounts/{id} owner setDefaultPayoutAccount
//
// # Set default payout account
//
// Marks the specified account as the default payout account. Only one account
// can be the default at a time; the previous default is cleared automatically.
// Requires role: owner.
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
//	403: errorResponse
//	404: errorResponse
func (h *Handler) setDefaultAccount(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")
	if err := h.svc.SetDefaultAccount(c.Request.Context(), ownerID, id); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deletePayoutAccount handles DELETE /owner/payout-accounts/:id.
//
// swagger:route DELETE /owner/payout-accounts/{id} owner deletePayoutAccount
//
// # Remove a payout account
//
// Soft-deletes a payout account. Requires role: owner.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	204: {}
//	401: errorResponse
//	403: errorResponse
//	404: errorResponse
func (h *Handler) deletePayoutAccount(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")
	if err := h.svc.DeletePayoutAccount(c.Request.Context(), id, ownerID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// verifyAccountOTP handles POST /owner/payout-accounts/:id/verify-otp.
//
// swagger:route POST /owner/payout-accounts/{id}/verify-otp owner verifyPayoutAccountOTP
//
// # Verify GCash or Maya payout account via OTP
//
// Verifies a GCash or Maya payout account using a 6-digit OTP sent to the
// registered mobile number. Bank accounts use admin verification instead.
// Requires role: owner.
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
//	403: errorResponse
//	404: errorResponse
func (h *Handler) verifyAccountOTP(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if err := h.svc.VerifyAccountOTP(c.Request.Context(), id, ownerID, req.OTP); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// listEarnings handles GET /owner/earnings.
//
// swagger:route GET /owner/earnings owner listOwnerEarnings
//
// # List owner earnings
//
// Returns earnings records for the authenticated owner. Filter by status using
// the ?status= query parameter (pending, released, disputed, paid_out).
// All amount fields are in centavos — divide by 100 for display in PHP.
// Requires role: owner.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: earningListResponse
//	401: errorResponse
//	403: errorResponse
func (h *Handler) listEarnings(c *gin.Context) {
	ownerID := c.GetString("user_id")
	status := c.Query("status")
	earnings, err := h.svc.GetEarnings(c.Request.Context(), ownerID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if earnings == nil {
		earnings = []OwnerEarning{}
	}
	c.JSON(http.StatusOK, earnings)
}

// listPayouts handles GET /owner/payouts.
//
// swagger:route GET /owner/payouts owner listOwnerPayouts
//
// # List owner payout history
//
// Returns all payout batches initiated for the authenticated owner.
// All amount fields are in centavos — divide by 100 for display in PHP.
// Requires role: owner.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: payoutListResponse
//	401: errorResponse
//	403: errorResponse
func (h *Handler) listPayouts(c *gin.Context) {
	ownerID := c.GetString("user_id")
	payouts, err := h.svc.GetPayouts(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if payouts == nil {
		payouts = []OwnerPayout{}
	}
	c.JSON(http.StatusOK, payouts)
}

// updatePayoutSchedule handles PUT /owner/payout-schedule.
//
// swagger:route PUT /owner/payout-schedule owner updatePayoutSchedule
//
// # Update payout schedule preference
//
// Sets the payout frequency for the authenticated owner.
// 'daily' runs at 2am Manila time every day; 'weekly' runs at 2am Manila time
// every Monday. Requires role: owner.
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
//	403: errorResponse
func (h *Handler) updatePayoutSchedule(c *gin.Context) {
	var req UpdatePayoutScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	ownerID := c.GetString("user_id")
	if err := h.svc.UpdatePayoutSchedule(c.Request.Context(), ownerID, req.Schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You do not own this resource"})
	case errors.Is(err, ErrNoVerifiedPayoutAccount):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no_verified_payout_account", "message": err.Error()})
	case errors.Is(err, ErrDisputeWindowClosed):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "dispute_window_closed", "message": err.Error()})
	case errors.Is(err, ErrAlreadyDisputed):
		c.JSON(http.StatusConflict, gin.H{"error": "already_disputed", "message": err.Error()})
	case errors.Is(err, ErrOTPNotSupported):
		c.JSON(http.StatusBadRequest, gin.H{"error": "otp_not_supported", "message": err.Error()})
	case errors.Is(err, ErrAccountAlreadyVerified):
		c.JSON(http.StatusConflict, gin.H{"error": "already_verified", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
