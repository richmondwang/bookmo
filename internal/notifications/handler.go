package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes HTTP endpoints for the notifications module.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes attaches the notification routes to the provided router group.
// All routes require the caller to be authenticated (user_id set in context).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/notifications", h.GetUnread)
	rg.POST("/notifications/read", h.MarkRead)
	rg.POST("/notifications/device-token", h.RegisterDeviceToken)
}

// GetUnread returns unread notifications for the authenticated user.
//
//	GET /notifications
//	Response: {"notifications": [...]}
func (h *Handler) GetUnread(c *gin.Context) {
	userID := c.GetString("user_id")

	notifications, err := h.svc.repo.GetUnread(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to retrieve notifications",
		})
		return
	}

	// Return an empty array rather than null when there are no notifications.
	if notifications == nil {
		notifications = []Notification{}
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// markReadRequest is the request body for the MarkRead endpoint.
type markReadRequest struct {
	IDs []string `json:"ids" binding:"required,min=1"`
}

// MarkRead marks the specified notifications as read for the authenticated user.
//
//	POST /notifications/read
//	Body: {"ids": ["uuid1", "uuid2"]}
func (h *Handler) MarkRead(c *gin.Context) {
	userID := c.GetString("user_id")

	var req markReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := h.svc.repo.MarkRead(c.Request.Context(), userID, req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to mark notifications as read",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterDeviceToken saves (or reactivates) a device token for the authenticated user.
//
//	POST /notifications/device-token
//	Body: {"token": "...", "platform": "ios"|"android"}
func (h *Handler) RegisterDeviceToken(c *gin.Context) {
	userID := c.GetString("user_id")

	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := h.svc.repo.SaveDeviceToken(c.Request.Context(), userID, req.Token, req.Platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to save device token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
