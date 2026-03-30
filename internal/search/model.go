package search

// SearchParams holds the parameters for a search or browse request.
type SearchParams struct {
	Lat          float64
	Lng          float64
	Query        string  // empty = browse mode
	CategorySlug string  // empty = all categories
	Radius       float64 // meters, default 5000
	Limit        int     // default 20, max 50
	Offset       int
}

// ServiceResult is a single item returned by a search or browse query.
type ServiceResult struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        *string  `json:"description,omitempty"`
	PricePerUnit       float64  `json:"price_per_unit"`
	MinDuration        int      `json:"min_duration"`
	MaxDuration        int      `json:"max_duration"`
	CapacityType       string   `json:"capacity_type"`
	AllowsParticipants bool     `json:"allows_participants"`
	BranchID           string   `json:"branch_id"`
	BranchName         string   `json:"branch_name"`
	BranchAddress      string   `json:"branch_address"`
	DistanceMeters     float64  `json:"distance_meters"`
	CategoryID         *string  `json:"category_id,omitempty"`
	CategoryName       *string  `json:"category_name,omitempty"`
	AvgRating          float64  `json:"avg_rating"`
	TotalReviews       int      `json:"total_reviews"`
	Tags               []string `json:"tags,omitempty"`
}

// Category represents a service category entry.
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	IconURL  *string `json:"icon_url,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
}

// SearchResponse wraps a list of results and a total count.
type SearchResponse struct {
	Results []ServiceResult `json:"results"`
	Total   int             `json:"total"`
}
