package profiles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds the HTTP handlers for the profiles module.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires up the profiles endpoints onto the provided router group.
// The group is expected to already have the RequireAuth middleware applied.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/users/:id/profile", h.GetProfile)
	rg.PUT("/users/me/profile", h.UpdateProfile)
	rg.POST("/users/me/photo/upload-url", h.GetPhotoUploadURL)
	rg.POST("/users/me/photo/confirm", h.ConfirmPhotoUpload)
}

// GetProfile handles GET /users/:id/profile
// Returns a role-aware profile response based on the caller's role.
func (h *Handler) GetProfile(c *gin.Context) {
	callerID := c.GetString("user_id")
	callerRole := c.GetString("user_role")
	targetUserID := c.Param("id")

	result, err := h.svc.GetProfile(c.Request.Context(), callerID, callerRole, targetUserID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "profile_not_found",
				"message": "Profile not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An unexpected error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateProfile handles PUT /users/me/profile
// Partially updates the authenticated user's own profile.
func (h *Handler) UpdateProfile(c *gin.Context) {
	callerID := c.GetString("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := h.svc.UpdateProfile(c.Request.Context(), callerID, &req); err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "profile_not_found",
				"message": "Profile not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An unexpected error occurred",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPhotoUploadURL handles POST /users/me/photo/upload-url
// Returns a pre-signed S3 upload URL and the final CDN URL.
func (h *Handler) GetPhotoUploadURL(c *gin.Context) {
	callerID := c.GetString("user_id")

	resp, err := h.svc.GeneratePhotoUploadURL(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "upload_url_failed",
			"message": "Failed to generate upload URL",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url": resp.UploadURL,
		"cdn_url":    resp.CDNURL,
	})
}

// ConfirmPhotoUpload handles POST /users/me/photo/confirm
// Validates the CDN URL and persists it to the user's profile.
func (h *Handler) ConfirmPhotoUpload(c *gin.Context) {
	callerID := c.GetString("user_id")

	var req ConfirmPhotoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if req.CDNURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "cdn_url is required",
		})
		return
	}

	if err := h.svc.ConfirmPhotoUpload(c.Request.Context(), callerID, req.CDNURL); err != nil {
		if errors.Is(err, ErrPhotoUploadFailed) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_cdn_url",
				"message": "The provided CDN URL is not valid for this platform",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An unexpected error occurred",
		})
		return
	}

	c.Status(http.StatusNoContent)
}
