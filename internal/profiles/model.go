package profiles

import "time"

// UserProfile is the full DB row from the users table (plus profile columns).
type UserProfile struct {
	ID              string
	Email           string
	Phone           *string
	Role            string
	FullName        *string
	Bio             *string
	ProfilePhotoURL *string
	IsVerified      bool
	CreatedAt       time.Time
	DeletedAt       *time.Time
}

// CustomerTrustProfile is the pre-aggregated trust row from customer_trust_profiles.
// It is trigger-owned — never written by application code.
type CustomerTrustProfile struct {
	CustomerID        string
	TotalBookings     int
	CompletedBookings int
	CancelledBookings int
	CompletionRate    float64
	CancellationRate  float64
	AvgOwnerRating    float64
	TotalOwnerReviews int
	LastUpdated       time.Time
}

// OwnerReviewOnProfile is a slim view of a customer_review used on profile pages.
type OwnerReviewOnProfile struct {
	ID          string
	OwnerID     string
	Rating      int
	Body        *string
	SubmittedAt time.Time
}

// BasicProfileResponse is returned to customers viewing another customer's profile.
type BasicProfileResponse struct {
	ID              string    `json:"id"`
	FullName        *string   `json:"full_name"`
	ProfilePhotoURL *string   `json:"profile_photo_url"`
	IsVerified      bool      `json:"is_verified"`
	JoinedAt        time.Time `json:"joined_at"`
}

// FullProfileResponse is returned to owners (full trust data) and to the profile owner (own data).
type FullProfileResponse struct {
	ID              string                 `json:"id"`
	FullName        *string                `json:"full_name"`
	ProfilePhotoURL *string                `json:"profile_photo_url"`
	Bio             *string                `json:"bio"`
	IsVerified      bool                   `json:"is_verified"`
	JoinedAt        time.Time              `json:"joined_at"`
	Trust           *CustomerTrustProfile  `json:"trust,omitempty"`
	OwnerReviews    []OwnerReviewOnProfile `json:"owner_reviews,omitempty"`
}

// UpdateProfileRequest is the HTTP request body for PUT /users/me/profile.
type UpdateProfileRequest struct {
	FullName *string `json:"full_name"`
	Bio      *string `json:"bio"`
	Phone    *string `json:"phone"`
}

// PhotoUploadURLResponse is returned from POST /users/me/photo/upload-url.
type PhotoUploadURLResponse struct {
	UploadURL string `json:"upload_url"`
	CDNURL    string `json:"cdn_url"`
}

// ConfirmPhotoRequest is the HTTP request body for POST /users/me/photo/confirm.
type ConfirmPhotoRequest struct {
	CDNURL string `json:"cdn_url"`
}
