package notifications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the notifications module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository returns a new Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateNotification inserts a notification row and populates n.ID and n.CreatedAt.
func (r *Repository) CreateNotification(ctx context.Context, n *Notification) error {
	const q = `
		INSERT INTO notifications (user_id, type, title, body, data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	err := r.db.QueryRow(ctx, q, n.UserID, n.Type, n.Title, n.Body, n.Data).
		Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		return fmt.Errorf("notifications.CreateNotification: %w", err)
	}
	return nil
}

// GetUnread returns the 50 most recent unread notifications for a user.
func (r *Repository) GetUnread(ctx context.Context, userID string) ([]Notification, error) {
	const q = `
		SELECT id, user_id, type, title, body, data, is_read, created_at
		FROM notifications
		WHERE user_id = $1
		  AND is_read = false
		ORDER BY created_at DESC
		LIMIT 50`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetUnread: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Data, &n.IsRead, &n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("notifications.GetUnread scan: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications.GetUnread rows: %w", err)
	}
	return out, nil
}

// MarkRead marks the given notification IDs as read, scoped to the user.
func (r *Repository) MarkRead(ctx context.Context, userID string, notificationIDs []string) error {
	const q = `
		UPDATE notifications
		SET is_read = true
		WHERE id = ANY($1)
		  AND user_id = $2`
	_, err := r.db.Exec(ctx, q, notificationIDs, userID)
	if err != nil {
		return fmt.Errorf("notifications.MarkRead: %w", err)
	}
	return nil
}

// GetActiveDeviceTokens returns all active device tokens for a user.
func (r *Repository) GetActiveDeviceTokens(ctx context.Context, userID string) ([]DeviceToken, error) {
	const q = `
		SELECT id, user_id, token, platform, is_active, created_at
		FROM device_tokens
		WHERE user_id = $1
		  AND is_active = true`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetActiveDeviceTokens: %w", err)
	}
	defer rows.Close()

	var out []DeviceToken
	for rows.Next() {
		var dt DeviceToken
		if err := rows.Scan(
			&dt.ID, &dt.UserID, &dt.Token, &dt.Platform, &dt.IsActive, &dt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("notifications.GetActiveDeviceTokens scan: %w", err)
		}
		out = append(out, dt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications.GetActiveDeviceTokens rows: %w", err)
	}
	return out, nil
}

// DeactivateToken marks a device token as inactive.
func (r *Repository) DeactivateToken(ctx context.Context, tokenID string) error {
	const q = `UPDATE device_tokens SET is_active = false WHERE id = $1`
	_, err := r.db.Exec(ctx, q, tokenID)
	if err != nil {
		return fmt.Errorf("notifications.DeactivateToken: %w", err)
	}
	return nil
}

// LogDeliveryAttempt inserts a notification delivery log row.
func (r *Repository) LogDeliveryAttempt(ctx context.Context, l *NotificationLog) error {
	const q = `
		INSERT INTO notification_logs
			(notification_id, device_token_id, status, provider_ref, error_message)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, attempted_at`
	err := r.db.QueryRow(ctx, q,
		l.NotificationID, l.DeviceTokenID, l.Status, l.ProviderRef, l.ErrorMessage,
	).Scan(&l.ID, &l.AttemptedAt)
	if err != nil {
		return fmt.Errorf("notifications.LogDeliveryAttempt: %w", err)
	}
	return nil
}

// SaveDeviceToken upserts a device token, reactivating it if it already exists.
func (r *Repository) SaveDeviceToken(ctx context.Context, userID, token, platform string) error {
	const q = `
		INSERT INTO device_tokens (user_id, token, platform)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, token) DO UPDATE
			SET is_active = true,
			    platform  = $3`
	_, err := r.db.Exec(ctx, q, userID, token, platform)
	if err != nil {
		return fmt.Errorf("notifications.SaveDeviceToken: %w", err)
	}
	return nil
}
