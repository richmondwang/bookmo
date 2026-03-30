package search

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the search module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers public search routes on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/search", h.search)
	rg.GET("/categories", h.getCategories)
	rg.GET("/search/suggestions", h.getSuggestions)
}

// search handles GET /search
//
// Query params:
//
//	lat      float64  required
//	lng      float64  required
//	q        string   optional  (empty = browse mode)
//	category string   optional  category slug
//	radius   float64  optional  metres, default 5000
//	limit    int      optional  default 20, max 50
//	offset   int      optional  default 0
func (h *Handler) search(c *gin.Context) {
	lat, ok := parseFloat(c, "lat")
	if !ok {
		return
	}
	lng, ok := parseFloat(c, "lng")
	if !ok {
		return
	}

	p := SearchParams{
		Lat:          lat,
		Lng:          lng,
		Query:        c.Query("q"),
		CategorySlug: c.Query("category"),
		Radius:       parseFloatOrDefault(c, "radius", defaultRadius),
		Limit:        parseIntOrDefault(c, "limit", defaultLimit),
		Offset:       parseIntOrDefault(c, "offset", 0),
	}

	resp, err := h.svc.Search(c.Request.Context(), p)
	if err != nil {
		if errors.Is(err, ErrInvalidCoordinates) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_coordinates",
				"message": "lat must be -90..90 and lng must be -180..180",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "an unexpected error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// getCategories handles GET /categories
func (h *Handler) getCategories(c *gin.Context) {
	cats, err := h.svc.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "an unexpected error occurred",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

// getSuggestions handles GET /search/suggestions
//
// Query params:
//
//	q string  the prefix to complete
func (h *Handler) getSuggestions(c *gin.Context) {
	prefix := c.Query("q")
	suggestions, err := h.svc.GetSuggestions(c.Request.Context(), prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "an unexpected error occurred",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// --- helpers ---

func parseFloat(c *gin.Context, param string) (float64, bool) {
	raw := c.Query(param)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_parameter",
			"message": param + " is required",
		})
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_parameter",
			"message": param + " must be a valid number",
		})
		return 0, false
	}
	return v, true
}

func parseFloatOrDefault(c *gin.Context, param string, def float64) float64 {
	raw := c.Query(param)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func parseIntOrDefault(c *gin.Context, param string, def int) int {
	raw := c.Query(param)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	return v
}
