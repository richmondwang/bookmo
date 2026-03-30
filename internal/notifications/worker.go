package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Worker consumes notification jobs from Redis and delivers push notifications.
type Worker struct {
	repo   *Repository
	rdb    *redis.Client
	fcmKey string
}

// NewWorker returns a Worker ready to consume from the notification job queue.
func NewWorker(repo *Repository, rdb *redis.Client, fcmKey string) *Worker {
	return &Worker{repo: repo, rdb: rdb, fcmKey: fcmKey}
}

// Consume blocks and processes jobs from the "notification_jobs" Redis list until ctx is cancelled.
func (w *Worker) Consume(ctx context.Context) error {
	log.Println("notifications.Worker: started, listening on", jobQueue)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// BRPOP blocks up to 2 seconds so we can check ctx regularly.
		results, err := w.rdb.BRPop(ctx, 2*time.Second, jobQueue).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				// Timeout — no message, loop again.
				continue
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			log.Printf("notifications.Worker: brpop error: %v", err)
			continue
		}

		// results[0] = key name, results[1] = payload
		if len(results) < 2 {
			continue
		}

		var job NotificationJob
		if err := json.Unmarshal([]byte(results[1]), &job); err != nil {
			log.Printf("notifications.Worker: unmarshal job: %v", err)
			continue
		}

		if err := w.Deliver(ctx, &job); err != nil {
			log.Printf("notifications.Worker: deliver job %s: %v", job.NotificationID, err)
		}
	}
}

// Deliver fetches the user's active device tokens and attempts push delivery to each.
func (w *Worker) Deliver(ctx context.Context, job *NotificationJob) error {
	tokens, err := w.repo.GetActiveDeviceTokens(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("notifications.Deliver get tokens: %w", err)
	}

	for _, dt := range tokens {
		w.deliverToToken(ctx, dt, job)
	}
	return nil
}

// deliverToToken attempts push delivery to a single device token with up to 3 attempts.
// Backoff: attempt 1 immediately, attempt 2 after 1 min, attempt 3 after 5 min.
// On invalid_token response, the token is deactivated immediately without retry.
func (w *Worker) deliverToToken(ctx context.Context, dt DeviceToken, job *NotificationJob) {
	backoffs := []time.Duration{0, 1 * time.Minute, 5 * time.Minute}
	const maxAttempts = 3

	notifID := job.NotificationID
	tokenID := dt.ID

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if backoffs[attempt] > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoffs[attempt]):
			}
		}

		providerRef, sendErr := w.sendPush(ctx, dt, job)

		entry := &NotificationLog{
			NotificationID: &notifID,
			DeviceTokenID:  &tokenID,
			ProviderRef:    providerRef,
		}

		if sendErr == nil {
			entry.Status = "sent"
			if logErr := w.repo.LogDeliveryAttempt(ctx, entry); logErr != nil {
				log.Printf("notifications.deliverToToken log sent: %v", logErr)
			}
			return
		}

		// Check for invalid token — deactivate and stop retrying.
		if errors.Is(sendErr, ErrInvalidToken) {
			entry.Status = "invalid_token"
			entry.ErrorMessage = sendErr.Error()
			if logErr := w.repo.LogDeliveryAttempt(ctx, entry); logErr != nil {
				log.Printf("notifications.deliverToToken log invalid_token: %v", logErr)
			}
			if deactErr := w.repo.DeactivateToken(ctx, dt.ID); deactErr != nil {
				log.Printf("notifications.deliverToToken deactivate token %s: %v", dt.ID, deactErr)
			}
			return
		}

		// Transient failure — log and maybe retry.
		entry.Status = "failed"
		entry.ErrorMessage = sendErr.Error()
		if logErr := w.repo.LogDeliveryAttempt(ctx, entry); logErr != nil {
			log.Printf("notifications.deliverToToken log failed: %v", logErr)
		}

		if attempt == maxAttempts-1 {
			log.Printf("notifications.deliverToToken: max attempts reached for token %s, job %s", dt.ID, job.NotificationID)
		}
	}
}

// sendPush delivers a push notification to a single device token.
// MVP stub: logs the attempt and returns success.
// A real implementation would call FCM for Android or APNs for iOS.
func (w *Worker) sendPush(ctx context.Context, token DeviceToken, job *NotificationJob) (providerRef string, err error) {
	// MVP stub: log and return success.
	// Real implementation would call FCM/APNs based on token.Platform.
	log.Printf("notifications.sendPush: platform=%s token=%s title=%q type=%s notif_id=%s",
		token.Platform, token.Token, job.Title, job.Type, job.NotificationID)
	return "stub-ref", nil
}
