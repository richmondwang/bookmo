package marketplace

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminHandler serves admin-only marketplace routes.
type AdminHandler struct {
	svc *Service
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(svc *Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// RegisterAdminRoutes registers admin routes on the given group.
// The caller must apply RequireAuth and RequireRole("admin") middleware.
func (h *AdminHandler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")
	admin.GET("/disputes", h.listDisputes)
	admin.POST("/disputes/:id/resolve", h.resolveDispute)
	admin.GET("/payout-accounts/pending-verification", h.listUnverifiedAccounts)
	admin.POST("/payout-accounts/:id/verify", h.verifyPayoutAccount)
	admin.POST("/fee-overrides", h.upsertFeeOverride)
	admin.PUT("/fee-overrides/:owner_id", h.upsertFeeOverride)
	admin.GET("/category-fee-rates", h.listCategoryFeeRates)
	admin.PUT("/category-fee-rates/:category_id", h.updateCategoryFeeRate)
	admin.GET("/platform-settings", h.getPlatformSettings)
	admin.PUT("/platform-settings", h.updatePlatformSettings)
}

// listDisputes handles GET /admin/disputes.
//
// swagger:route GET /admin/disputes owner listDisputes
//
// # List booking disputes
//
// Returns all booking disputes. Filter by status using ?status= (open, under_review,
// resolved_release, resolved_refund). Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: disputeListResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) listDisputes(c *gin.Context) {
	status := c.Query("status")
	disputes, err := h.svc.repo.ListDisputes(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if disputes == nil {
		disputes = []BookingDispute{}
	}
	c.JSON(http.StatusOK, disputes)
}

// resolveDispute handles POST /admin/disputes/:id/resolve.
//
// swagger:route POST /admin/disputes/{id}/resolve owner resolveDispute
//
// # Resolve a booking dispute
//
// Resolves an open dispute as either 'release' (payout proceeds to owner) or
// 'refund' (customer is refunded, owner earning is removed). Requires admin role.
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
func (h *AdminHandler) resolveDispute(c *gin.Context) {
	id := c.Param("id")
	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if err := h.svc.AdminResolveDispute(c.Request.Context(), id, &req); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// listUnverifiedAccounts handles GET /admin/payout-accounts/pending-verification.
//
// swagger:route GET /admin/payout-accounts/pending-verification owner listUnverifiedPayoutAccounts
//
// # List payout accounts pending verification
//
// Returns all unverified payout accounts across all owners. Used by admins to
// manually verify bank accounts. Requires admin role.
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
func (h *AdminHandler) listUnverifiedAccounts(c *gin.Context) {
	accounts, err := h.svc.repo.ListUnverifiedPayoutAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if accounts == nil {
		accounts = []OwnerPayoutAccount{}
	}
	c.JSON(http.StatusOK, accounts)
}

// verifyPayoutAccount handles POST /admin/payout-accounts/:id/verify.
//
// swagger:route POST /admin/payout-accounts/{id}/verify owner adminVerifyPayoutAccount
//
// # Manually verify a payout account
//
// Marks a payout account as verified. Used for bank accounts that require manual
// admin review (GCash/Maya accounts use OTP verification instead). Requires admin role.
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
func (h *AdminHandler) verifyPayoutAccount(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.AdminVerifyPayoutAccount(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// upsertFeeOverride handles POST and PUT /admin/fee-overrides.
//
// swagger:route POST /admin/fee-overrides owner upsertOwnerFeeOverride
//
// # Create or update an owner fee override
//
// Sets a negotiated platform fee for a specific owner. Fee type 'percent' uses
// basis points (1000 = 10%); fee type 'flat' uses centavos. Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: feeOverrideResponse
//	400: errorResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) upsertFeeOverride(c *gin.Context) {
	var req UpsertFeeOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	override, err := h.svc.repo.UpsertFeeOverride(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	c.JSON(http.StatusOK, override)
}

// listCategoryFeeRates handles GET /admin/category-fee-rates.
//
// swagger:route GET /admin/category-fee-rates owner listCategoryFeeRates
//
// # List category fee rates
//
// Returns all category-level platform fee rates. Fee values are in basis points
// (1000 = 10%). Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: categoryFeeRateListResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) listCategoryFeeRates(c *gin.Context) {
	rates, err := h.svc.repo.ListCategoryFeeRates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	if rates == nil {
		rates = []CategoryFeeRate{}
	}
	c.JSON(http.StatusOK, rates)
}

// updateCategoryFeeRate handles PUT /admin/category-fee-rates/:category_id.
//
// swagger:route PUT /admin/category-fee-rates/{category_id} owner updateCategoryFeeRate
//
// # Set category fee rate
//
// Creates or updates the platform fee for a service category. Fee value is in
// basis points (1000 = 10%). Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: categoryFeeRateListResponse
//	400: errorResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) updateCategoryFeeRate(c *gin.Context) {
	categoryID := c.Param("category_id")
	var req UpdateCategoryFeeRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	rate, err := h.svc.repo.UpsertCategoryFeeRate(c.Request.Context(), categoryID, req.FeePercentBP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	c.JSON(http.StatusOK, rate)
}

// getPlatformSettings handles GET /admin/platform-settings.
//
// swagger:route GET /admin/platform-settings owner getPlatformSettings
//
// # Get platform settings
//
// Returns the global platform settings including default fee in basis points and
// the dispute window duration in hours. Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: platformSettingsResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) getPlatformSettings(c *gin.Context) {
	settings, err := h.svc.repo.GetPlatformSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// updatePlatformSettings handles PUT /admin/platform-settings.
//
// swagger:route PUT /admin/platform-settings owner updatePlatformSettings
//
// # Update platform settings
//
// Updates the global default fee (basis points) and/or dispute window duration
// (hours). All fields are optional — omitted fields retain their current value.
// Requires admin role.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	200: platformSettingsResponse
//	400: errorResponse
//	401: errorResponse
//	403: errorResponse
func (h *AdminHandler) updatePlatformSettings(c *gin.Context) {
	var req UpdatePlatformSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.svc.repo.UpdatePlatformSettings(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *AdminHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Record not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
