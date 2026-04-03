package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all auth-related database operations.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetUserByEmail retrieves a user by email. Returns ErrNotFound when absent.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, role, full_name, email_verified, deleted_at, created_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`
	u := &User{}
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.FullName,
		&u.EmailVerified, &u.DeletedAt, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth.GetUserByEmail: %w", err)
	}
	return u, nil
}

// GetUserByID retrieves a user by primary key. Returns ErrNotFound when absent.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, role, full_name, email_verified, deleted_at, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`
	u := &User{}
	err := r.db.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.FullName,
		&u.EmailVerified, &u.DeletedAt, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth.GetUserByID: %w", err)
	}
	return u, nil
}

// CreateUser inserts a new user and returns it. email_verified defaults to the
// provided value (true for SSO users, false for email/password users).
func (r *Repository) CreateUser(ctx context.Context, email, role string, fullName *string, emailVerified bool) (*User, error) {
	const q = `
		INSERT INTO users (email, role, full_name, email_verified)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, role, full_name, email_verified, deleted_at, created_at`
	u := &User{}
	err := r.db.QueryRow(ctx, q, email, role, fullName, emailVerified).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.FullName,
		&u.EmailVerified, &u.DeletedAt, &u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auth.CreateUser: %w", err)
	}
	return u, nil
}

// SetPassword updates a user's password hash.
func (r *Repository) SetPassword(ctx context.Context, userID, hash string) error {
	const q = `UPDATE users SET password_hash = $1 WHERE id = $2 AND deleted_at IS NULL`
	if _, err := r.db.Exec(ctx, q, hash, userID); err != nil {
		return fmt.Errorf("auth.SetPassword: %w", err)
	}
	return nil
}

// GetIdentityByProvider retrieves a user_identity row. Returns ErrNotFound when absent.
func (r *Repository) GetIdentityByProvider(ctx context.Context, provider, providerID string) (*UserIdentity, error) {
	const q = `
		SELECT id, user_id, provider, provider_id, email, created_at
		FROM user_identities
		WHERE provider = $1 AND provider_id = $2`
	id := &UserIdentity{}
	err := r.db.QueryRow(ctx, q, provider, providerID).Scan(
		&id.ID, &id.UserID, &id.Provider, &id.ProviderID, &id.Email, &id.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth.GetIdentityByProvider: %w", err)
	}
	return id, nil
}

// CreateIdentity inserts a new user_identity row.
func (r *Repository) CreateIdentity(ctx context.Context, userID, provider, providerID string, email *string) error {
	const q = `
		INSERT INTO user_identities (user_id, provider, provider_id, email)
		VALUES ($1, $2, $3, $4)`
	if _, err := r.db.Exec(ctx, q, userID, provider, providerID, email); err != nil {
		return fmt.Errorf("auth.CreateIdentity: %w", err)
	}
	return nil
}

// LinkIdentity adds a new SSO identity to an existing user account.
func (r *Repository) LinkIdentity(ctx context.Context, userID, provider, providerID string, email *string) error {
	return r.CreateIdentity(ctx, userID, provider, providerID, email)
}
