package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	otpTTL         = 10 * time.Minute
	pendingLinkTTL = 15 * time.Minute
	maxOTPAttempts = 3
	otpLength      = 6
)

// pendingLinkPayload is stored in Redis under sso:pending_link:{token}.
type pendingLinkPayload struct {
	UserID        string `json:"user_id"`
	Provider      string `json:"provider"`
	ProviderID    string `json:"provider_id"`
	ProviderEmail string `json:"provider_email"`
}

// otpPayload is stored in Redis under auth:otp:{email}.
type otpPayload struct {
	Hash     string `json:"hash"`
	Attempts int    `json:"attempts"`
}

// Service holds auth business logic.
type Service struct {
	repo      *Repository
	verifier  *Verifier
	rdb       *redis.Client
	jwtSecret string
}

// NewService constructs a Service.
func NewService(repo *Repository, verifier *Verifier, rdb *redis.Client, jwtSecret string) *Service {
	return &Service{repo: repo, verifier: verifier, rdb: rdb, jwtSecret: jwtSecret}
}

// IssueJWT creates a signed JWT for the given user.
func (s *Service) IssueJWT(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(24 * time.Hour * 30).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("auth.IssueJWT: %w", err)
	}
	return signed, nil
}

// AuthenticateSSO handles the full SSO flow:
// 1. Verify provider token → extract claims (discard token immediately after)
// 2. If identity exists → issue JWT (returning user)
// 3. If email is new → create user + identity → issue JWT (new user)
// 4. If email collision → store pending link → return ErrEmailCollision
func (s *Service) AuthenticateSSO(ctx context.Context, req *SSORequest) (string, error) {
	claims, err := s.verifier.Verify(ctx, req.Provider, req.Token)
	if err != nil {
		return "", fmt.Errorf("auth.AuthenticateSSO: %w", err)
	}
	// Token verified — discard raw token; work only with extracted claims from here.

	// 1. Returning user via this identity.
	identity, err := s.repo.GetIdentityByProvider(ctx, req.Provider, claims.ProviderID)
	if err == nil {
		user, err := s.repo.GetUserByID(ctx, identity.UserID)
		if err != nil {
			return "", fmt.Errorf("auth.AuthenticateSSO: get user: %w", err)
		}
		return s.IssueJWT(user.ID, user.Role)
	}
	if !errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("auth.AuthenticateSSO: lookup identity: %w", err)
	}

	// 2. Check if the email already exists.
	existingUser, err := s.repo.GetUserByEmail(ctx, claims.Email)
	if errors.Is(err, ErrNotFound) {
		// New user — create account and identity.
		role := req.Role
		if role == "" {
			role = "customer"
		}
		var fullName *string
		if n := s.buildName(req, claims); n != "" {
			fullName = &n
		}
		user, err := s.repo.CreateUser(ctx, claims.Email, role, fullName, true)
		if err != nil {
			return "", fmt.Errorf("auth.AuthenticateSSO: create user: %w", err)
		}
		providerEmail := claims.Email
		if err := s.repo.CreateIdentity(ctx, user.ID, req.Provider, claims.ProviderID, &providerEmail); err != nil {
			return "", fmt.Errorf("auth.AuthenticateSSO: create identity: %w", err)
		}
		return s.IssueJWT(user.ID, user.Role)
	}
	if err != nil {
		return "", fmt.Errorf("auth.AuthenticateSSO: lookup email: %w", err)
	}

	// 3. Email collision — store pending link and return 409.
	token, err := s.StorePendingLink(ctx, existingUser.ID, req.Provider, claims.ProviderID, claims.Email)
	if err != nil {
		return "", fmt.Errorf("auth.AuthenticateSSO: store pending link: %w", err)
	}
	return "", &ErrEmailCollision{PendingLinkToken: token}
}

// buildName constructs a full_name string from the SSO request and provider claims.
// Apple name is used only on first sign-in; Google/Facebook use the claims name.
func (s *Service) buildName(req *SSORequest, claims *ProviderClaims) string {
	if req.Provider == "apple" && req.AppleName != nil {
		parts := []string{req.AppleName.GivenName, req.AppleName.FamilyName}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
	return claims.Name
}

// StorePendingLink writes a short-lived Redis key for an email-collision link flow.
// Returns a cryptographically random hex token.
func (s *Service) StorePendingLink(ctx context.Context, userID, provider, providerID, providerEmail string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth.StorePendingLink: rand: %w", err)
	}
	token := hex.EncodeToString(raw)

	payload := pendingLinkPayload{
		UserID:        userID,
		Provider:      provider,
		ProviderID:    providerID,
		ProviderEmail: providerEmail,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("auth.StorePendingLink: marshal: %w", err)
	}
	key := "sso:pending_link:" + token
	if err := s.rdb.Set(ctx, key, data, pendingLinkTTL).Err(); err != nil {
		return "", fmt.Errorf("auth.StorePendingLink: redis: %w", err)
	}
	return token, nil
}

// VerifyPendingLink resolves a pending link using the supplied OTP.
// After successful verification the new SSO identity is linked to the user's account.
func (s *Service) VerifyPendingLink(ctx context.Context, req *VerifyLinkRequest) (string, error) {
	key := "sso:pending_link:" + req.PendingLinkToken
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("auth.VerifyPendingLink: redis get: %w", err)
	}

	var payload pendingLinkPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("auth.VerifyPendingLink: unmarshal: %w", err)
	}

	if req.OTP != "" {
		// OTP verification path.
		user, err := s.repo.GetUserByID(ctx, payload.UserID)
		if err != nil {
			return "", fmt.Errorf("auth.VerifyPendingLink: get user: %w", err)
		}
		if err := s.checkOTP(ctx, user.Email, req.OTP); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("auth.VerifyPendingLink: OTP or provider_token required")
	}

	// Link the new identity.
	providerEmail := payload.ProviderEmail
	if err := s.repo.LinkIdentity(ctx, payload.UserID, payload.Provider, payload.ProviderID, &providerEmail); err != nil {
		return "", fmt.Errorf("auth.VerifyPendingLink: link identity: %w", err)
	}

	// Delete the pending link so it can only be used once.
	_ = s.rdb.Del(ctx, key)

	user, err := s.repo.GetUserByID(ctx, payload.UserID)
	if err != nil {
		return "", fmt.Errorf("auth.VerifyPendingLink: get user post-link: %w", err)
	}
	return s.IssueJWT(user.ID, user.Role)
}

// SendOTP generates a 6-digit OTP, hashes it with bcrypt, and stores it in Redis.
// Always returns nil — callers must return 200 regardless of whether the email exists.
func (s *Service) SendOTP(ctx context.Context, email string) error {
	// Generate 6-digit OTP.
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		log.Printf("auth.SendOTP: rand: %v", err)
		return nil // do not leak errors
	}
	otpStr := fmt.Sprintf("%06d", n.Int64())

	hash, err := bcrypt.GenerateFromPassword([]byte(otpStr), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("auth.SendOTP: bcrypt: %v", err)
		return nil
	}

	payload := otpPayload{Hash: string(hash), Attempts: 0}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("auth.SendOTP: marshal: %v", err)
		return nil
	}

	key := "auth:otp:" + email
	if err := s.rdb.Set(ctx, key, data, otpTTL).Err(); err != nil {
		log.Printf("auth.SendOTP: redis: %v", err)
		return nil
	}

	// In production: call email delivery service here.
	// In development: log the OTP for testing.
	log.Printf("[DEV] OTP for %s: %s", email, otpStr)
	return nil
}

// checkOTP validates the OTP for the given email, incrementing and enforcing the attempt limit.
func (s *Service) checkOTP(ctx context.Context, email, otp string) error {
	key := "auth:otp:" + email
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrInvalidOTP
	}
	if err != nil {
		return fmt.Errorf("auth.checkOTP: redis get: %w", err)
	}

	var payload otpPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("auth.checkOTP: unmarshal: %w", err)
	}

	if payload.Attempts >= maxOTPAttempts {
		_ = s.rdb.Del(ctx, key)
		return ErrOTPMaxAttempts
	}

	if err := bcrypt.CompareHashAndPassword([]byte(payload.Hash), []byte(otp)); err != nil {
		payload.Attempts++
		if payload.Attempts >= maxOTPAttempts {
			// Delete on max attempts — user must request a new OTP.
			_ = s.rdb.Del(ctx, key)
			return ErrOTPMaxAttempts
		}
		// Save updated attempt count.
		if data, merr := json.Marshal(payload); merr == nil {
			ttl := s.rdb.TTL(ctx, key).Val()
			_ = s.rdb.Set(ctx, key, data, ttl)
		}
		return ErrInvalidOTP
	}

	// Correct OTP — delete so it cannot be reused.
	_ = s.rdb.Del(ctx, key)
	return nil
}

// VerifyOTP validates an OTP and returns a JWT for the matching user.
func (s *Service) VerifyOTP(ctx context.Context, req *VerifyOTPRequest) (string, error) {
	if err := s.checkOTP(ctx, req.Email, req.OTP); err != nil {
		return "", err
	}
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, ErrNotFound) {
		return "", ErrInvalidOTP
	}
	if err != nil {
		return "", fmt.Errorf("auth.VerifyOTP: get user: %w", err)
	}
	return s.IssueJWT(user.ID, user.Role)
}

// SetPassword hashes and stores a new password for the authenticated user.
func (s *Service) SetPassword(ctx context.Context, userID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth.SetPassword: bcrypt: %w", err)
	}
	if err := s.repo.SetPassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("auth.SetPassword: %w", err)
	}
	return nil
}
