package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ProviderClaims are the identity fields extracted from a verified provider token.
type ProviderClaims struct {
	ProviderID string
	Email      string
	Name       string
}

// Verifier handles provider-token verification for all three SSO providers.
// Each method is a production-grade HTTP call (Google, Facebook) or a JWT
// signature stub (Apple) that should be upgraded with full JWKS verification
// before going live.
type Verifier struct {
	httpClient *http.Client
}

// NewVerifier creates a Verifier with the default HTTP client.
func NewVerifier() *Verifier {
	return &Verifier{httpClient: &http.Client{}}
}

// VerifyGoogleToken verifies a Google id_token using Google's tokeninfo endpoint.
func (v *Verifier) VerifyGoogleToken(ctx context.Context, token string) (*ProviderClaims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://oauth2.googleapis.com/tokeninfo?id_token="+url.QueryEscape(token), nil)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyGoogleToken: build request: %w", err)
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyGoogleToken: %w: %w", ErrInvalidProviderToken, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidProviderToken
	}
	var payload struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("auth.VerifyGoogleToken: decode: %w", ErrInvalidProviderToken)
	}
	if payload.Sub == "" || payload.Email == "" {
		return nil, ErrInvalidProviderToken
	}
	return &ProviderClaims{ProviderID: payload.Sub, Email: payload.Email, Name: payload.Name}, nil
}

// VerifyFacebookToken verifies a Facebook access_token via the Graph API.
func (v *Verifier) VerifyFacebookToken(ctx context.Context, token string) (*ProviderClaims, error) {
	endpoint := "https://graph.facebook.com/me?fields=id,email,name&access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyFacebookToken: build request: %w", err)
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyFacebookToken: %w: %w", ErrInvalidProviderToken, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidProviderToken
	}
	var payload struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("auth.VerifyFacebookToken: decode: %w", ErrInvalidProviderToken)
	}
	if payload.ID == "" || payload.Email == "" {
		return nil, ErrInvalidProviderToken
	}
	return &ProviderClaims{ProviderID: payload.ID, Email: payload.Email, Name: payload.Name}, nil
}

// VerifyAppleToken decodes and validates an Apple identity_token.
// NOTE: This implementation decodes the JWT payload without verifying the RS256
// signature. In production, fetch Apple's JWKS from https://appleid.apple.com/auth/keys,
// cache the keys in Redis with their expiry TTL, and verify the signature before
// trusting any claims.
func (v *Verifier) VerifyAppleToken(ctx context.Context, token string) (*ProviderClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidProviderToken
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidProviderToken
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, ErrInvalidProviderToken
	}
	if claims.Sub == "" || claims.Email == "" {
		return nil, ErrInvalidProviderToken
	}
	return &ProviderClaims{ProviderID: claims.Sub, Email: claims.Email}, nil
}

// Verify dispatches token verification to the correct provider.
func (v *Verifier) Verify(ctx context.Context, provider, token string) (*ProviderClaims, error) {
	switch provider {
	case "google":
		return v.VerifyGoogleToken(ctx, token)
	case "facebook":
		return v.VerifyFacebookToken(ctx, token)
	case "apple":
		return v.VerifyAppleToken(ctx, token)
	default:
		return nil, ErrInvalidProviderToken
	}
}
