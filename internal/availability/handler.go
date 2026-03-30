package availability

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for the availability module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes attaches availability routes to the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/availability", h.GetSlots)
}

// GetSlots handles GET /availability?branch_id=&service_id=&date=YYYY-MM-DD
// and returns available slots for the requested service on the requested date.
func (h *Handler) GetSlots(c *gin.Context) {
	var req GetSlotsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_date",
			"message": "date must be in YYYY-MM-DD format",
		})
		return
	}

	slots, err := h.svc.GetSlotsForDate(c.Request.Context(), req.BranchID, req.ServiceID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to retrieve availability slots",
		})
		return
	}

	resp := make([]SlotResponse, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, SlotResponse{
			StartTime:         s.StartTime.UTC().Format(time.RFC3339),
			EndTime:           s.EndTime.UTC().Format(time.RFC3339),
			RemainingCapacity: s.RemainingCapacity,
		})
	}

	c.JSON(http.StatusOK, gin.H{"slots": resp})
}
