package notifications

import (
	"encoding/json"
	"time"
)

// Notification is a persisted in-app notification row.
type Notification struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data,omitempty"`
	IsRead    bool            `json:"is_read"`
	CreatedAt time.Time       `json:"created_at"`
}

// DeviceToken is a persisted device token row.
type DeviceToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationLog is a persisted delivery attempt row.
type NotificationLog struct {
	ID             string    `json:"id"`
	NotificationID *string   `json:"notification_id,omitempty"`
	DeviceTokenID  *string   `json:"device_token_id,omitempty"`
	Status         string    `json:"status"`
	ProviderRef    string    `json:"provider_ref,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

// NotificationJob is the payload serialized to Redis and consumed by the worker.
type NotificationJob struct {
	NotificationID string         `json:"notification_id"`
	UserID         string         `json:"user_id"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	Data           map[string]any `json:"data,omitempty"`
	Type           string         `json:"type"`
}

// RegisterTokenRequest is the HTTP request body for registering a device token.
type RegisterTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required,oneof=ios android"`
}
