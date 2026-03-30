package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/richmondwang/bookmo/pkg/config"
	"github.com/richmondwang/bookmo/pkg/db"
	redispkg "github.com/richmondwang/bookmo/pkg/redis"
)

func Run(cfg *config.Config) error {
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("worker: db: %w", err)
	}
	defer pool.Close()

	rdb, err := redispkg.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("worker: redis: %w", err)
	}
	defer rdb.Close()

	_ = pool
	_ = rdb

	log.Println("Worker started")

	// Main loop — tick every minute for scheduler jobs
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// TODO: run scheduler jobs
			// scheduler.FireApprovalDeadlineWarnings(ctx, pool, rdb)
			// scheduler.FireReminders(ctx, pool, rdb)
			// scheduler.AutoCancelExpiredBookings(ctx, pool, rdb)
			// scheduler.ProcessNotificationQueue(ctx, pool, rdb)
		}
	}
}
