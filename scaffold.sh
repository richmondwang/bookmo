#!/usr/bin/env bash
# scaffold.sh — Run this once to create the full project directory structure
# Usage: bash scaffold.sh
set -e

echo "Creating Kadto — a Booking Platform project structure..."

# ── cmd ──────────────────────────────────────────────────────────────────────
mkdir -p cmd/api cmd/worker

cat > cmd/api/main.go << 'EOF'
package main

import (
	"log"
	"github.com/richmondwang/kadto/internal/config"
	"github.com/richmondwang/kadto/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := server.Run(cfg); err != nil {
		log.Fatalf("server: %v", err)
	}
}
EOF

cat > cmd/worker/main.go << 'EOF'
package main

import (
	"log"
	"github.com/richmondwang/kadto/internal/config"
	"github.com/richmondwang/kadto/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := worker.Run(cfg); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
EOF

# ── internal modules ──────────────────────────────────────────────────────────
MODULES=(auth users owners services availability bookings payments notifications reviews search scheduler)

for mod in "${MODULES[@]}"; do
	mkdir -p "internal/$mod"
	# model.go
	cat > "internal/$mod/model.go" << EOF
package $mod
EOF
	# repository.go
	cat > "internal/$mod/repository.go" << EOF
package $mod

import "github.com/jackc/pgx/v5/pgxpool"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}
EOF
	# service.go
	cat > "internal/$mod/service.go" << EOF
package $mod

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}
EOF
	# handler.go
	cat > "internal/$mod/handler.go" << EOF
package $mod

import "github.com/gin-gonic/gin"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// TODO: register routes
}
EOF
	# errors.go
	cat > "internal/$mod/errors.go" << EOF
package $mod

import "errors"

var (
	ErrNotFound = errors.New("${mod}: not found")
)
EOF
	echo "  created internal/$mod/"
done

# ── pkg ───────────────────────────────────────────────────────────────────────
mkdir -p pkg/db pkg/redis pkg/config pkg/middleware

cat > pkg/config/config.go << 'EOF'
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	AppEnv string
	Port   string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// JWT
	JWTSecret string

	// PayMongo
	PayMongoSecretKey      string
	PayMongoWebhookSecret  string

	// Firebase / APNs
	FCMServerKey  string
	APNSKeyID     string
	APNSTeamID    string
	APNSBundleID  string

	// Storage
	S3Bucket string
	S3Region string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env is optional; env vars take precedence

	cfg := &Config{
		AppEnv:                os.Getenv("APP_ENV"),
		Port:                  getEnvOrDefault("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		PayMongoSecretKey:     os.Getenv("PAYMONGO_SECRET_KEY"),
		PayMongoWebhookSecret: os.Getenv("PAYMONGO_WEBHOOK_SECRET"),
		FCMServerKey:          os.Getenv("FCM_SERVER_KEY"),
		APNSKeyID:             os.Getenv("APNS_KEY_ID"),
		APNSTeamID:            os.Getenv("APNS_TEAM_ID"),
		APNSBundleID:          os.Getenv("APNS_BUNDLE_ID"),
		S3Bucket:              os.Getenv("S3_BUCKET"),
		S3Region:              getEnvOrDefault("S3_REGION", "ap-southeast-1"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
EOF

cat > pkg/db/db.go << 'EOF'
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db.Connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db.Connect ping: %w", err)
	}
	return pool, nil
}
EOF

cat > pkg/redis/redis.go << 'EOF'
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis.Connect parse: %w", err)
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis.Connect ping: %w", err)
	}
	return client, nil
}
EOF

cat > pkg/middleware/auth.go << 'EOF'
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		claims, _ := token.Claims.(jwt.MapClaims)
		c.Set("user_id", claims["sub"])
		c.Set("user_role", claims["role"])
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("user_role") != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
EOF

# ── internal/server ───────────────────────────────────────────────────────────
mkdir -p internal/server

cat > internal/server/server.go << 'EOF'
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/richmondwang/kadto/pkg/config"
	"github.com/richmondwang/kadto/pkg/db"
	redispkg "github.com/richmondwang/kadto/pkg/redis"
)

func Run(cfg *config.Config) error {
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("server: db: %w", err)
	}
	defer pool.Close()

	rdb, err := redispkg.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("server: redis: %w", err)
	}
	defer rdb.Close()

	_ = pool
	_ = rdb

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	// TODO: register module route groups here
	// v1 := r.Group("/v1")
	// authHandler.RegisterRoutes(v1)
	// ...

	return r.Run(":" + cfg.Port)
}
EOF

# ── internal/worker ───────────────────────────────────────────────────────────
mkdir -p internal/worker

cat > internal/worker/worker.go << 'EOF'
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/richmondwang/kadto/pkg/config"
	"github.com/richmondwang/kadto/pkg/db"
	redispkg "github.com/richmondwang/kadto/pkg/redis"
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
EOF

# ── migrations ────────────────────────────────────────────────────────────────
mkdir -p migrations

cat > migrations/001_extensions.up.sql << 'EOF'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
EOF

cat > migrations/001_extensions.down.sql << 'EOF'
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS postgis;
DROP EXTENSION IF EXISTS "uuid-ossp";
EOF

cat > migrations/002_users.up.sql << 'EOF'
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email         TEXT UNIQUE NOT NULL,
  phone         TEXT,
  password_hash TEXT,
  role          TEXT NOT NULL CHECK (role IN ('customer','owner','admin')),
  deleted_at    TIMESTAMP,
  created_at    TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
EOF

cat > migrations/002_users.down.sql << 'EOF'
DROP TABLE IF EXISTS users;
EOF

cat > migrations/003_owners_branches.up.sql << 'EOF'
CREATE TABLE owners (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id             UUID NOT NULL REFERENCES users(id),
  business_name       TEXT NOT NULL,
  verification_status TEXT NOT NULL DEFAULT 'pending'
                        CHECK (verification_status IN ('pending','verified','rejected')),
  onboarding_step     TEXT NOT NULL DEFAULT 'profile'
                        CHECK (onboarding_step IN ('profile','branch','service','availability','complete')),
  deleted_at          TIMESTAMP,
  created_at          TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE branches (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id   UUID NOT NULL REFERENCES owners(id),
  name       TEXT NOT NULL,
  address    TEXT NOT NULL,
  location   GEOGRAPHY(POINT, 4326) NOT NULL,
  deleted_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_branches_owner ON branches(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_branches_location ON branches USING GIST(location);
EOF

cat > migrations/003_owners_branches.down.sql << 'EOF'
DROP TABLE IF EXISTS branches;
DROP TABLE IF EXISTS owners;
EOF

cat > migrations/004_categories_services.up.sql << 'EOF'
CREATE TABLE categories (
  id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name      TEXT NOT NULL,
  slug      TEXT UNIQUE NOT NULL,
  icon_url  TEXT,
  parent_id UUID REFERENCES categories(id),
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE services (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id       UUID NOT NULL REFERENCES branches(id),
  category_id     UUID REFERENCES categories(id),
  name            TEXT NOT NULL,
  description     TEXT,
  min_duration    INT NOT NULL,
  max_duration    INT NOT NULL,
  step_minutes    INT NOT NULL DEFAULT 30,
  capacity        INT NOT NULL DEFAULT 1,
  capacity_type   TEXT NOT NULL CHECK (capacity_type IN ('single','multi')),
  price_per_unit  NUMERIC(12,2) NOT NULL,
  tags            TEXT[],
  search_vec      TSVECTOR,
  deleted_at      TIMESTAMP,
  created_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_services_branch   ON services(branch_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_category ON services(category_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_search   ON services USING GIN(search_vec);
CREATE INDEX idx_services_tags     ON services USING GIN(tags);

CREATE FUNCTION update_services_search_vec() RETURNS TRIGGER AS $$
BEGIN
  NEW.search_vec := to_tsvector('english',
    NEW.name || ' ' || coalesce(NEW.description, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_services_search_vec
BEFORE INSERT OR UPDATE ON services
FOR EACH ROW EXECUTE FUNCTION update_services_search_vec();
EOF

cat > migrations/004_categories_services.down.sql << 'EOF'
DROP TRIGGER IF EXISTS trg_services_search_vec ON services;
DROP FUNCTION IF EXISTS update_services_search_vec;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS categories;
EOF

cat > migrations/005_availability.up.sql << 'EOF'
CREATE TABLE availability_rules (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id   UUID NOT NULL REFERENCES branches(id),
  day_of_week INT  NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
  start_time  TIME NOT NULL,
  end_time    TIME NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE date_overrides (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id   UUID NOT NULL REFERENCES branches(id),
  date        DATE NOT NULL,
  is_closed   BOOLEAN NOT NULL DEFAULT false,
  open_time   TIME,
  close_time  TIME,
  note        TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (branch_id, date)
);

CREATE INDEX idx_availability_rules_branch ON availability_rules(branch_id);
CREATE INDEX idx_date_overrides_branch_date ON date_overrides(branch_id, date);
EOF

cat > migrations/005_availability.down.sql << 'EOF'
DROP TABLE IF EXISTS date_overrides;
DROP TABLE IF EXISTS availability_rules;
EOF

cat > migrations/006_bookings.up.sql << 'EOF'
CREATE TABLE bookings (
  id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id                  UUID NOT NULL REFERENCES services(id),
  branch_id                   UUID NOT NULL REFERENCES branches(id),
  customer_id                 UUID NOT NULL REFERENCES users(id),
  start_time                  TIMESTAMP NOT NULL,
  end_time                    TIMESTAMP NOT NULL,
  quantity                    INT NOT NULL DEFAULT 1,
  status                      TEXT NOT NULL CHECK (status IN (
                                'pending','awaiting_approval','confirmed',
                                'rejected','cancelled','rescheduled','completed'
                              )),
  payment_method              TEXT CHECK (payment_method IN ('card','gcash','maya','bank_transfer')),
  owner_response_deadline     TIMESTAMP,
  rescheduled_from_booking_id UUID REFERENCES bookings(id),
  reschedule_attempt_count    INT NOT NULL DEFAULT 0,
  rejected_reason             TEXT,
  cancelled_by                TEXT CHECK (cancelled_by IN ('customer','owner','system')),
  currency                    TEXT NOT NULL DEFAULT 'PHP',
  deleted_at                  TIMESTAMP,
  created_at                  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE booking_locks (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id UUID NOT NULL REFERENCES services(id),
  branch_id  UUID NOT NULL REFERENCES branches(id),
  start_time TIMESTAMP NOT NULL,
  end_time   TIMESTAMP NOT NULL,
  quantity   INT NOT NULL DEFAULT 1,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE reschedule_requests (
  id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id     UUID NOT NULL REFERENCES bookings(id),
  requested_by   UUID NOT NULL REFERENCES users(id),
  new_start_time TIMESTAMP NOT NULL,
  new_end_time   TIMESTAMP NOT NULL,
  status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','approved','rejected')),
  created_at     TIMESTAMP NOT NULL DEFAULT now()
);

-- Enforce one pending reschedule per booking
CREATE UNIQUE INDEX idx_one_pending_reschedule
  ON reschedule_requests (booking_id)
  WHERE status = 'pending';

-- GiST index for overlap queries
CREATE INDEX idx_bookings_time
  ON bookings USING GIST (tstzrange(start_time, end_time));

CREATE INDEX idx_bookings_service_status
  ON bookings(service_id, status) WHERE deleted_at IS NULL;

CREATE INDEX idx_bookings_customer
  ON bookings(customer_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_booking_locks_expiry
  ON booking_locks(expires_at);
EOF

cat > migrations/006_bookings.down.sql << 'EOF'
DROP TABLE IF EXISTS reschedule_requests;
DROP TABLE IF EXISTS booking_locks;
DROP TABLE IF EXISTS bookings;
EOF

cat > migrations/007_payments.up.sql << 'EOF'
CREATE TABLE payment_intents (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id       UUID NOT NULL REFERENCES bookings(id),
  paymongo_id      TEXT UNIQUE NOT NULL,
  amount_centavos  INT NOT NULL,
  currency         TEXT NOT NULL DEFAULT 'PHP',
  method           TEXT CHECK (method IN ('card','gcash','maya','bank_transfer')),
  method_type      TEXT CHECK (method_type IN ('auth_capture','immediate_capture')),
  status           TEXT NOT NULL CHECK (status IN (
                     'pending','authorized','captured','voided','refunded','failed'
                   )),
  paymongo_status  TEXT,
  captured_at      TIMESTAMP,
  voided_at        TIMESTAMP,
  created_at       TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE refunds (
  id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  payment_intent_id  UUID NOT NULL REFERENCES payment_intents(id),
  booking_id         UUID NOT NULL REFERENCES bookings(id),
  paymongo_refund_id TEXT UNIQUE,
  amount_centavos    INT NOT NULL,
  reason             TEXT CHECK (reason IN (
                       'owner_rejected','owner_cancelled','customer_cancelled',
                       'system_timeout','dispute'
                     )),
  status             TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')),
  created_at         TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE webhook_events (
  id           TEXT PRIMARY KEY,
  type         TEXT,
  processed_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE cancellation_policies (
  id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id         UUID NOT NULL REFERENCES services(id),
  hours_before_start INT NOT NULL,
  refund_percent     INT NOT NULL CHECK (refund_percent BETWEEN 0 AND 100),
  created_at         TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_intents_booking ON payment_intents(booking_id);
CREATE INDEX idx_refunds_booking ON refunds(booking_id);
EOF

cat > migrations/007_payments.down.sql << 'EOF'
DROP TABLE IF EXISTS cancellation_policies;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS payment_intents;
EOF

cat > migrations/008_notifications.up.sql << 'EOF'
CREATE TABLE device_tokens (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token      TEXT NOT NULL,
  platform   TEXT NOT NULL CHECK (platform IN ('ios','android')),
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (user_id, token)
);

CREATE TABLE notifications (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type       TEXT NOT NULL,
  title      TEXT NOT NULL,
  body       TEXT NOT NULL,
  data       JSONB,
  is_read    BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE notification_logs (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  notification_id UUID REFERENCES notifications(id),
  device_token_id UUID REFERENCES device_tokens(id),
  status          TEXT NOT NULL CHECK (status IN ('pending','sent','failed','invalid_token')),
  provider_ref    TEXT,
  error_message   TEXT,
  attempted_at    TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE review_prompts (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id       UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  sent_at          TIMESTAMP,
  reminder_sent_at TIMESTAMP,
  review_id        UUID,
  created_at       TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user   ON notifications(user_id, is_read);
CREATE INDEX idx_device_tokens_user   ON device_tokens(user_id) WHERE is_active = true;
CREATE INDEX idx_notification_logs_status ON notification_logs(status, attempted_at);
EOF

cat > migrations/008_notifications.down.sql << 'EOF'
DROP TABLE IF EXISTS review_prompts;
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS device_tokens;
EOF

cat > migrations/009_reviews.up.sql << 'EOF'
CREATE TABLE reviews (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id   UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  service_id   UUID NOT NULL REFERENCES services(id),
  branch_id    UUID NOT NULL REFERENCES branches(id),
  customer_id  UUID NOT NULL REFERENCES users(id),
  rating       SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body         TEXT CHECK (char_length(body) <= 1000),
  is_anonymous BOOLEAN NOT NULL DEFAULT false,
  status       TEXT NOT NULL DEFAULT 'published'
                 CHECK (status IN ('published','flagged','removed')),
  submitted_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE review_responses (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  review_id  UUID NOT NULL REFERENCES reviews(id) UNIQUE,
  owner_id   UUID NOT NULL REFERENCES owners(id),
  body       TEXT NOT NULL CHECK (char_length(body) <= 500),
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP
);

CREATE TABLE review_flags (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  review_id   UUID NOT NULL REFERENCES reviews(id),
  reported_by UUID NOT NULL REFERENCES users(id),
  reason      TEXT CHECK (reason IN ('spam','offensive','fake','irrelevant')),
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (review_id, reported_by)
);

CREATE TABLE rating_summaries (
  service_id    UUID PRIMARY KEY REFERENCES services(id),
  total_reviews INT NOT NULL DEFAULT 0,
  total_rating  INT NOT NULL DEFAULT 0,
  avg_rating    NUMERIC(2,1) NOT NULL DEFAULT 0,
  five_star     INT NOT NULL DEFAULT 0,
  four_star     INT NOT NULL DEFAULT 0,
  three_star    INT NOT NULL DEFAULT 0,
  two_star      INT NOT NULL DEFAULT 0,
  one_star      INT NOT NULL DEFAULT 0,
  last_updated  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_reviews_service ON reviews(service_id, status);
CREATE INDEX idx_reviews_customer ON reviews(customer_id);

-- Trigger to keep rating_summaries current
CREATE FUNCTION update_rating_summary() RETURNS TRIGGER AS $$
DECLARE col TEXT;
BEGIN
  col := CASE NEW.rating
    WHEN 5 THEN 'five_star' WHEN 4 THEN 'four_star'
    WHEN 3 THEN 'three_star' WHEN 2 THEN 'two_star'
    ELSE 'one_star' END;

  IF TG_OP = 'INSERT' THEN
    INSERT INTO rating_summaries (service_id, total_reviews, total_rating, avg_rating)
    VALUES (NEW.service_id, 1, NEW.rating, NEW.rating)
    ON CONFLICT (service_id) DO UPDATE SET
      total_reviews = rating_summaries.total_reviews + 1,
      total_rating  = rating_summaries.total_rating + NEW.rating,
      avg_rating    = ROUND((rating_summaries.total_rating + NEW.rating)::numeric
                       / (rating_summaries.total_reviews + 1), 1),
      last_updated  = now();
    EXECUTE format('UPDATE rating_summaries SET %I = %I + 1 WHERE service_id = $1', col, col)
      USING NEW.service_id;

  ELSIF TG_OP = 'UPDATE' AND OLD.status != 'removed' AND NEW.status = 'removed' THEN
    UPDATE rating_summaries SET
      total_reviews = GREATEST(total_reviews - 1, 0),
      total_rating  = GREATEST(total_rating - OLD.rating, 0),
      avg_rating    = CASE WHEN total_reviews - 1 = 0 THEN 0
                      ELSE ROUND((total_rating - OLD.rating)::numeric / (total_reviews - 1), 1)
                      END,
      last_updated  = now()
    WHERE service_id = OLD.service_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_rating_summary
AFTER INSERT OR UPDATE ON reviews
FOR EACH ROW EXECUTE FUNCTION update_rating_summary();
EOF

cat > migrations/009_reviews.down.sql << 'EOF'
DROP TRIGGER IF EXISTS trg_rating_summary ON reviews;
DROP FUNCTION IF EXISTS update_rating_summary;
DROP TABLE IF EXISTS rating_summaries;
DROP TABLE IF EXISTS review_flags;
DROP TABLE IF EXISTS review_responses;
DROP TABLE IF EXISTS reviews;
EOF

# ── .env.example ─────────────────────────────────────────────────────────────
cat > .env.example << 'EOF'
# App
APP_ENV=development
PORT=8080

# PostgreSQL — must have PostGIS and pg_trgm installed
DATABASE_URL=postgres://postgres:password@localhost:5432/booking_platform?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# JWT — generate with: openssl rand -hex 32
JWT_SECRET=change_me_in_production

# PayMongo (Philippines)
PAYMONGO_SECRET_KEY=sk_test_...
PAYMONGO_WEBHOOK_SECRET=whsec_...

# FCM (Android push)
FCM_SERVER_KEY=...

# APNs (iOS push)
APNS_KEY_ID=...
APNS_TEAM_ID=...
APNS_BUNDLE_ID=com.skylerlabs.kadtobookingplatform

# S3-compatible storage
S3_BUCKET=booking-platform-media
S3_REGION=ap-southeast-1
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_ENDPOINT=  # leave empty for AWS; set for Cloudflare R2 or MinIO
EOF

# ── .claude/settings.json ─────────────────────────────────────────────────────
mkdir -p .claude

cat > .claude/settings.json << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "[ \"$(git branch --show-current)\" != \"main\" ] || { echo '{\"block\": true, \"message\": \"Cannot edit directly on main branch — create a feature branch first\"}' >&2; exit 2; }",
            "timeout": 5
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "which gofmt >/dev/null 2>&1 && gofmt -w . 2>/dev/null || true",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
EOF

# ── docs ──────────────────────────────────────────────────────────────────────
mkdir -p docs

cat > docs/openapi.yaml << 'EOF'
openapi: 3.1.0
info:
  title: Kadto — a Booking Platform API
  version: 1.0.0
  description: |
    Two-sided booking marketplace for the Philippines market.
    Customers discover and book services. Owners manage listings and approve bookings.

servers:
  - url: http://localhost:8080/v1
    description: Local development
  - url: https://api.yourdomain.com/v1
    description: Production

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

  schemas:
    Error:
      type: object
      properties:
        error:   { type: string }
        message: { type: string }

    Booking:
      type: object
      properties:
        id:           { type: string, format: uuid }
        service_id:   { type: string, format: uuid }
        branch_id:    { type: string, format: uuid }
        customer_id:  { type: string, format: uuid }
        start_time:   { type: string, format: date-time }
        end_time:     { type: string, format: date-time }
        quantity:     { type: integer }
        status:
          type: string
          enum: [pending, awaiting_approval, confirmed, rejected, cancelled, rescheduled, completed]
        currency:     { type: string, example: PHP }
        created_at:   { type: string, format: date-time }

security:
  - bearerAuth: []

paths:
  /health:
    get:
      summary: Health check
      security: []
      responses:
        '200':
          description: OK

  # Auth
  /auth/register:
    post:
      summary: Register a new user
      security: []
      tags: [Auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email, password, role]
              properties:
                email:    { type: string, format: email }
                password: { type: string, minLength: 8 }
                phone:    { type: string }
                role:     { type: string, enum: [customer, owner] }
      responses:
        '201': { description: Created }
        '400': { description: Validation error }
        '409': { description: Email already registered }

  /auth/login:
    post:
      summary: Login
      security: []
      tags: [Auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email, password]
              properties:
                email:    { type: string }
                password: { type: string }
      responses:
        '200':
          description: JWT token
          content:
            application/json:
              schema:
                type: object
                properties:
                  token: { type: string }
        '401': { description: Invalid credentials }

  # Search
  /search:
    get:
      summary: Search services by location and keyword
      security: []
      tags: [Search]
      parameters:
        - name: lat
          in: query
          required: true
          schema: { type: number }
        - name: lng
          in: query
          required: true
          schema: { type: number }
        - name: q
          in: query
          schema: { type: string }
        - name: category
          in: query
          schema: { type: string }
        - name: radius
          in: query
          schema: { type: integer, default: 5000 }
        - name: page
          in: query
          schema: { type: integer, default: 0 }
      responses:
        '200': { description: List of services }

  # Availability
  /availability:
    get:
      summary: Get available slots for a service on a date
      tags: [Availability]
      parameters:
        - name: service_id
          in: query
          required: true
          schema: { type: string, format: uuid }
        - name: branch_id
          in: query
          required: true
          schema: { type: string, format: uuid }
        - name: date
          in: query
          required: true
          schema: { type: string, format: date }
      responses:
        '200': { description: Available slots }

  # Bookings
  /bookings/lock:
    post:
      summary: Lock a slot before payment
      tags: [Bookings]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [service_id, branch_id, start_time, end_time]
              properties:
                service_id:  { type: string, format: uuid }
                branch_id:   { type: string, format: uuid }
                start_time:  { type: string, format: date-time }
                end_time:    { type: string, format: date-time }
                quantity:    { type: integer, default: 1 }
      responses:
        '201': { description: Lock created }
        '409': { description: Slot unavailable }

  /bookings:
    post:
      summary: Create a booking (after payment intent confirmed)
      tags: [Bookings]
      responses:
        '201':
          description: Booking created
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Booking' }
    get:
      summary: List my bookings
      tags: [Bookings]
      responses:
        '200': { description: Booking list }

  /bookings/{id}/cancel:
    post:
      summary: Cancel a booking
      tags: [Bookings]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '200': { description: Cancelled }
        '404': { description: Not found }

  /bookings/{id}/reschedule:
    post:
      summary: Request a reschedule
      tags: [Bookings]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [new_start_time, new_end_time]
              properties:
                new_start_time: { type: string, format: date-time }
                new_end_time:   { type: string, format: date-time }
      responses:
        '201': { description: Reschedule request submitted }
        '409': { description: Pending reschedule already exists }
        '422': { description: Attempt limit reached }

  # Owner — approval queue
  /owner/queue:
    get:
      summary: Get pending bookings and reschedule requests
      tags: [Owner]
      responses:
        '200': { description: Approval queue items }

  /owner/bookings/{id}/approve:
    post:
      summary: Approve a booking
      tags: [Owner]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '200': { description: Approved }
        '409': { description: Slot no longer available }

  /owner/bookings/{id}/reject:
    post:
      summary: Reject a booking
      tags: [Owner]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                reason: { type: string }
      responses:
        '200': { description: Rejected }

  /owner/reschedules/{id}/approve:
    post:
      summary: Approve a reschedule request
      tags: [Owner]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '200': { description: Reschedule approved, new booking created }
        '409': { description: New slot unavailable }

  /owner/reschedules/{id}/reject:
    post:
      summary: Reject a reschedule request
      tags: [Owner]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '200': { description: Rejected, original booking stays confirmed }

  # Payments
  /payments/intent:
    post:
      summary: Create a PayMongo payment intent
      tags: [Payments]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [booking_id, method]
              properties:
                booking_id: { type: string, format: uuid }
                method:     { type: string, enum: [card, gcash, maya] }
      responses:
        '201':
          description: Payment intent created
          content:
            application/json:
              schema:
                type: object
                properties:
                  client_key:         { type: string }
                  payment_intent_id:  { type: string }

  /payments/webhook:
    post:
      summary: PayMongo webhook receiver
      security: []
      tags: [Payments]
      responses:
        '200': { description: Acknowledged }

  # Reviews
  /reviews:
    post:
      summary: Submit a review for a completed booking
      tags: [Reviews]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [booking_id, rating]
              properties:
                booking_id:   { type: string, format: uuid }
                rating:       { type: integer, minimum: 1, maximum: 5 }
                body:         { type: string, maxLength: 1000 }
                is_anonymous: { type: boolean, default: false }
      responses:
        '201': { description: Review submitted }
        '409': { description: Review already exists for this booking }
        '422': { description: Booking not completed or outside 14-day window }

  /services/{id}/reviews:
    get:
      summary: Get reviews for a service
      security: []
      tags: [Reviews]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
        - name: page
          in: query
          schema: { type: integer, default: 0 }
      responses:
        '200': { description: Review list with responses }
EOF

echo ""
echo "✅ Project structure created successfully."
echo ""
echo "Next steps:"
echo "  1. Replace 'github.com/richmondwang/kadto' in go.mod and all *.go files with your actual module path"
echo "  2. cp .env.example .env  and fill in your credentials"
echo "  3. go mod tidy"
echo "  4. Start Claude Code:  claude"
echo "  5. First prompt:  'Set up the database, run all migrations, then implement the auth module'"
EOF