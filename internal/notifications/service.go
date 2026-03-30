package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const jobQueue = "notification_jobs"

// Service orchestrates in-app notification creation and push job enqueueing.
type Service struct {
	repo *Repository
	rdb  *redis.Client
}

// NewService returns a Service backed by the given repository and Redis client.
func NewService(repo *Repository, rdb *redis.Client) *Service {
	return &Service{repo: repo, rdb: rdb}
}

// Enqueue serializes job to JSON and pushes it onto the Redis notification job queue.
func (s *Service) Enqueue(ctx context.Context, job *NotificationJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("notifications.Enqueue marshal: %w", err)
	}
	if err := s.rdb.LPush(ctx, jobQueue, payload).Err(); err != nil {
		return fmt.Errorf("notifications.Enqueue lpush: %w", err)
	}
	return nil
}

// CreateInAppNotification writes a notification row to the DB and enqueues a push job.
// The DB write is synchronous; push delivery is async via the worker.
func (s *Service) CreateInAppNotification(
	ctx context.Context,
	userID, notifType, title, body string,
	data map[string]any,
) (*Notification, error) {
	var rawData json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("notifications.CreateInAppNotification marshal data: %w", err)
		}
		rawData = b
	}

	n := &Notification{
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
		Data:   rawData,
	}
	if err := s.repo.CreateNotification(ctx, n); err != nil {
		return nil, fmt.Errorf("notifications.CreateInAppNotification: %w", err)
	}

	job := &NotificationJob{
		NotificationID: n.ID,
		UserID:         userID,
		Title:          title,
		Body:           body,
		Data:           data,
		Type:           notifType,
	}
	if err := s.Enqueue(ctx, job); err != nil {
		// Log but do not fail — the in-app notification was already persisted.
		// The push is best-effort.
		_ = err
	}

	return n, nil
}
