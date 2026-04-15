package profiles

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/richmondwang/kadto/pkg/config"
)

// Service implements the business logic for the profiles module.
type Service struct {
	repo *Repository
	cfg  *config.Config
}

func NewService(repo *Repository, cfg *config.Config) *Service {
	return &Service{repo: repo, cfg: cfg}
}

// GetProfile returns a role-aware profile response:
//   - owner caller       → FullProfileResponse with trust + owner reviews
//   - caller == target   → FullProfileResponse with own data (no trust object for non-owners)
//   - customer → other   → BasicProfileResponse
func (s *Service) GetProfile(ctx context.Context, callerID, callerRole, targetUserID string) (any, error) {
	profile, err := s.repo.GetProfile(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("profiles.GetProfile: %w", err)
	}

	isOwner := callerRole == "owner"
	isSelf := callerID == targetUserID

	if !isOwner && !isSelf {
		// Customer viewing another customer — return basic fields only.
		return &BasicProfileResponse{
			ID:              profile.ID,
			FullName:        profile.FullName,
			ProfilePhotoURL: profile.ProfilePhotoURL,
			IsVerified:      profile.IsVerified,
			JoinedAt:        profile.CreatedAt,
		}, nil
	}

	resp := &FullProfileResponse{
		ID:              profile.ID,
		FullName:        profile.FullName,
		ProfilePhotoURL: profile.ProfilePhotoURL,
		Bio:             profile.Bio,
		IsVerified:      profile.IsVerified,
		JoinedAt:        profile.CreatedAt,
	}

	if isOwner {
		// Owners get the full trust object and owner reviews array.
		trust, err := s.repo.GetTrustProfile(ctx, targetUserID)
		if err != nil && !errors.Is(err, ErrProfileNotFound) {
			return nil, fmt.Errorf("profiles.GetProfile trust: %w", err)
		}
		resp.Trust = trust // nil if customer has no trust row yet

		ownerReviews, err := s.repo.GetOwnerReviewsForCustomer(ctx, targetUserID)
		if err != nil {
			return nil, fmt.Errorf("profiles.GetProfile reviews: %w", err)
		}
		if ownerReviews == nil {
			ownerReviews = []OwnerReviewOnProfile{}
		}
		resp.OwnerReviews = ownerReviews
	}

	return resp, nil
}

// UpdateProfile applies a partial update to the caller's own profile.
func (s *Service) UpdateProfile(ctx context.Context, callerID string, req *UpdateProfileRequest) error {
	if err := s.repo.UpdateProfile(ctx, callerID, req); err != nil {
		return fmt.Errorf("profiles.UpdateProfile: %w", err)
	}
	return nil
}

// GeneratePhotoUploadURL returns a pre-signed S3 upload URL and the final CDN URL.
// For MVP the upload URL and CDN URL are the same public S3 URL; a real implementation
// would call the AWS SDK to generate a time-limited pre-signed PUT URL.
func (s *Service) GeneratePhotoUploadURL(ctx context.Context, userID string) (*PhotoUploadURLResponse, error) {
	photoKey := fmt.Sprintf("profile-photos/%s/%s.jpg", userID, generateUUID())

	cdnURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		s.cfg.S3Bucket, s.cfg.S3Region, photoKey)

	// In production this would be a time-limited pre-signed PUT URL from the AWS SDK.
	// For MVP we return the public CDN URL as both values so the confirm step works.
	uploadURL := cdnURL

	return &PhotoUploadURLResponse{
		UploadURL: uploadURL,
		CDNURL:    cdnURL,
	}, nil
}

// ConfirmPhotoUpload validates the CDN URL then persists it to the user row.
// We validate that the URL belongs to our configured S3 bucket to prevent
// callers from setting arbitrary URLs.
func (s *Service) ConfirmPhotoUpload(ctx context.Context, userID, cdnURL string) error {
	expected := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/profile-photos/",
		s.cfg.S3Bucket, s.cfg.S3Region)

	if !strings.HasPrefix(cdnURL, expected) {
		return ErrPhotoUploadFailed
	}

	if err := s.repo.SetPhotoURL(ctx, userID, cdnURL); err != nil {
		return fmt.Errorf("profiles.ConfirmPhotoUpload: %w", err)
	}
	return nil
}

// generateUUID returns a random UUID v4 string.
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use nanosecond timestamp — acceptable for a storage key.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
