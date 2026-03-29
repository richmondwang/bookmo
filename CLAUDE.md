# BookMo Booking Platform — Claude Code Context

## Project overview

A two-sided booking marketplace for the Philippines market. Customers discover and book services or places. Owners manage listings, approve bookings, and control availability. Built in Go, targeting 10k–100k users.

## Supporting documentation

Before implementing any module, read the relevant files below. Use `@filename` in your Claude Code prompt to load them into context.

- **@docs/domain-glossary.md** — precise definitions for every domain term. Use these exact names for variables, functions, and columns.
- **@docs/decisions/ADR-001-payment-method-split.md** — why GCash/Maya capture immediately but cards use auth/capture
- **@docs/decisions/ADR-002-reschedule-keeps-original-confirmed.md** — why the original booking stays confirmed during a reschedule
- **@docs/decisions/ADR-003-rating-summaries-trigger.md** — why rating summaries are owned by a DB trigger, not application code
- **@docs/decisions/ADR-004-geohash-feed-cache.md** — why Geohash precision-5 and how cache invalidation works
- **@docs/decisions/ADR-005-availability-resolution-priority.md** — why date overrides always beat weekly rules
- **@docs/decisions/ADR-006-async-notifications.md** — why push notifications are never sent in request handlers
- **@docs/decisions/ADR-007-dual-review-system-and-profiles.md** — owner reviews of customers, trust profiles, profile photos, role-aware visibility
- **@docs/decisions/ADR-008-booking-participants.md** — participant tagging, accept/decline flow, "booked with" history
- **@docs/decisions/ADR-009-participant-eligibility.md** — category default + service override, resolution priority, eligibility query

Read the relevant ADR before implementing: payments → ADR-001, bookings/reschedule → ADR-002, reviews → ADR-003, search → ADR-004, availability → ADR-005, notifications → ADR-006, profiles/customer-reviews → ADR-007, participants → ADR-008 + ADR-009.

---

## Stack

- **Language**: Go 1.22+ (modules enabled)
- **Framework**: Gin (`github.com/gin-gonic/gin`)
- **Database**: PostgreSQL 15+ with PostGIS and pg_trgm extensions
- **Cache / queues**: Redis 7+
- **Realtime**: gorilla/websocket
- **Payments**: PayMongo (Philippines — GCash, Maya, cards)
- **Push notifications**: FCM (Android) + APNs (iOS)
- **File storage**: S3-compatible (images, media)
- **Auth**: JWT (golang-jwt/jwt)

## Project structure

```
booking-platform/
├── cmd/
│   ├── api/              # Main HTTP server entrypoint
│   └── worker/           # Notification + scheduler worker entrypoint
├── internal/
│   ├── auth/             # JWT generation, middleware, role checks
│   ├── users/            # User registration, profile
│   ├── owners/           # Owner onboarding, branch management
│   ├── services/         # Service CRUD, category taxonomy
│   ├── availability/     # Slot generation, conflict detection, date overrides
│   ├── bookings/         # Booking state machine, locks, reschedule flow
│   ├── payments/         # PayMongo integration, webhook handler, refunds
│   ├── notifications/    # Push (FCM/APNs), in-app feed, job queue
│   ├── reviews/          # Review submission, moderation, rating summaries
│   ├── search/           # PostGIS proximity, FTS, Redis feed cache
│   ├── scheduler/        # Cron jobs: reminders, approval deadlines, reconciliation
│   ├── profiles/         # User profiles, photo upload, customer trust profiles
│   ├── customer_reviews/ # Owner reviews of customers, disputes
│   └── participants/     # Booking participant tagging, booked-with history
├── pkg/
│   ├── db/               # PostgreSQL connection pool, migration runner
│   ├── redis/            # Redis client wrapper
│   ├── config/           # Env-based config struct
│   └── middleware/       # Rate limiting, logging, CORS
├── migrations/           # SQL migration files (numbered, sequential)
├── docs/                 # OpenAPI spec (openapi.yaml)
├── .claude/
│   └── commands/         # Custom slash commands
├── .env.example
├── go.mod
├── go.sum
└── CLAUDE.md
```

Each module under `internal/` follows a consistent three-layer pattern:
- `handler.go` — Gin route handlers, request binding, response formatting
- `service.go` — Business logic, orchestration, validation
- `repository.go` — All SQL queries, no business logic here
- `model.go` — Structs for DB rows, request/response DTOs

## Commands

```bash
# Run the API server
go run ./cmd/api

# Run the background worker
go run ./cmd/worker

# Run all tests
go test ./...

# Run tests for a specific module
go test ./internal/bookings/...

# Run with race detector (always use for concurrency work)
go test -race ./...

# Apply database migrations
go run ./cmd/api migrate up

# Rollback last migration
go run ./cmd/api migrate down

# Build binaries
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker

# Lint
golangci-lint run ./...
```

## Database conventions

- All primary keys are UUID (`uuid_generate_v4()`) — never serial integers
- All tables have `created_at TIMESTAMP DEFAULT now()`
- All mutable entities have `deleted_at TIMESTAMP` — soft deletes only, never hard DELETE
- Monetary amounts are stored in centavos (integer), never decimal pesos
- Timestamps are stored in UTC; convert to `Asia/Manila` at the API layer for display
- Use `tstzrange` for booking overlap queries with a GiST index
- PostGIS `geography(POINT, 4326)` on `branches.location` — not `geometry`

## Key domain decisions (do not change without discussion)

**Booking state machine** — exactly 7 states, in order:
`pending` → `awaiting_approval` → `confirmed` → `completed`
Side exits: `rejected` (from awaiting_approval), `cancelled` (from confirmed), `rescheduled` (from confirmed, replaced by new booking)

**Payment split by method:**
- Cards: authorize on booking, capture on owner approval, void on rejection/timeout
- GCash / Maya: capture immediately on booking, full refund on rejection/timeout
- All amounts go through PayMongo — never store raw card data

**Reschedule flow:**
- Original booking stays `confirmed` while a reschedule request is `pending`
- Max 3 reschedule attempts per booking (`reschedule_attempt_count` on bookings)
- Enforced by partial unique index: `CREATE UNIQUE INDEX idx_one_pending_reschedule ON reschedule_requests (booking_id) WHERE status = 'pending'`
- New booking on approval is auto-confirmed (no second approval cycle)
- Slot availability must be re-checked inside the approval transaction

**Availability resolution priority (highest to lowest):**
1. `date_overrides` — one-off blocked dates or special hours
2. `availability_rules` — recurring weekly schedule
3. Closed (no slots returned)

**Owner approval window:** 24 hours from `booking.created_at`. Warn owner at 22h (2h remaining). Auto-cancel and refund at 24h via scheduler.

**Reviews:** One per completed booking (UNIQUE on `booking_id`). 14-day submission window enforced by CHECK constraint. `rating_summaries` updated via trigger — never recompute with live AVG().

**Search ranking:** Browse mode (no keyword) sorts by distance * 0.6 − rating * 0.4. Search mode sorts by FTS relevance + rating * 0.2. Cache browse results in Redis using Geohash precision-5 grid keys with 5-minute TTL.

## Profiles and customer reviews

- **Profile photos** use pre-signed S3 URLs — never stream file bytes through the API server. Store only the final CDN URL in `users.profile_photo_url`, never the upload URL.
- **`customer_trust_profiles` is trigger-owned** — same rule as `rating_summaries`. Never write to it from application code. Never compute trust signals from live queries — always read from this table.
- **Profile API is role-aware**: `GET /users/:id/profile` returns different fields based on caller role. Owners get the full trust object and `owner_reviews` array. Customers viewing another customer get only basic fields. This check belongs in the service layer, not the handler.
- **Customer reviews** follow the same 14-day window and one-per-booking rules as service reviews. UNIQUE on `booking_id` in `customer_reviews`.
- **Disputes do not hide reviews** — a disputed review stays published until an admin explicitly removes it by setting `customer_reviews.status = 'removed'`.
- **Approval queue must include trust data** — `GET /owner/queue` JOINs `customer_trust_profiles` and includes a `customer_trust` object on each item.
- **Completion rate denominator** counts only bookings that reached `confirmed`, `completed`, or `cancelled` — not `pending` or `awaiting_approval`.

## Booking participants

- **Eligibility is resolved at invite time** — call `ResolveParticipantEligibility` before any other validation in `participants.Invite`. Return `ErrParticipantsNotAllowed` (HTTP 403) if false.
- **Resolution order**: `services.allows_participants` → `categories.allows_participants` → parent category `allows_participants` → `false` (system default). `NULL` means not set — never treat NULL as false.
- **`GET /services/:id` exposes the resolved boolean** — compute and return `allows_participants: bool` (the effective value, not the raw nullable) so the UI can show/hide the invite button without a separate call.
- **Owners can override per service** — `services.allows_participants` is set in the owner service edit flow. If left unset (NULL), the category default applies automatically and inherits any future admin changes to the category.
- **Admins set category defaults** — `categories.allows_participants` is seeded per the Philippines taxonomy in migration 012. True for sports, recreation, venues. False for beauty, personal services.


- **Participants are record-only** — they have zero effect on booking state, payment, reviews, or the owner approval queue. Never check participant count or status in any booking, payment, or review logic.
- **Only the booking creator can invite** — check `booking.customer_id == caller.user_id` before inserting. Return `ErrNotBookingCreator` otherwise.
- **Cannot add or leave once completed** — check `booking.status != 'completed'` before any participant mutation. Return `ErrBookingCompleted` otherwise.
- **Creator cannot invite themselves** — check `user_id != booking.customer_id`. Return `ErrCannotInviteSelf`.
- **UNIQUE constraint handles duplicates** — `(booking_id, user_id)` is unique. Catch the constraint violation and return `ErrAlreadyInvited`.
- **Only accepted, non-left rows count for "booked with"** — always filter `status = 'accepted' AND left_at IS NULL AND b.status = 'completed'` in social history queries.
- **Two notification types**: `participant_invited` (to invited user) and `participant_accepted` (to creator). Both are async via the notification queue per ADR-006.

## Error handling conventions

- Return errors, never panic (except truly unrecoverable startup failures)
- Wrap errors with context: `fmt.Errorf("bookings.Confirm: %w", err)`
- Sentinel errors live in each module's `errors.go`:
  ```go
  var ErrBookingNotFound = errors.New("booking not found")
  var ErrSlotUnavailable = errors.New("slot unavailable")
  var ErrRescheduleLimitReached = errors.New("reschedule attempt limit reached")
  ```
- HTTP error responses always use this shape:
  ```json
  { "error": "slot_unavailable", "message": "The selected time slot is no longer available" }
  ```

## Testing conventions

- Unit tests live alongside source files: `service_test.go` next to `service.go`
- Integration tests that touch the DB go in `internal/<module>/integration_test.go` with build tag `//go:build integration`
- Use `testcontainers-go` to spin up a real PostgreSQL + Redis for integration tests
- Table-driven tests for all business logic — especially the state machine transitions
- Mock the repository layer in unit tests using interfaces

## Security rules

- JWT secret loaded from env, never hardcoded
- PayMongo webhook signature verified before processing — reject unsigned requests with 401
- Webhook events deduplicated via `webhook_events` table — always check before processing
- `device_tokens.is_active` set to false immediately on FCM/APNs `invalid_token` response
- Never send `customer_id` in API responses for anonymous reviews
- All owner-facing endpoints require role check: `middleware.RequireRole("owner")`
- Rate limiting: 100 req/min per IP for public endpoints, 1000 req/min for authenticated

## Important gotchas

- **PayMongo amounts in centavos**: ₱100 = `10000`. Always multiply by 100 before sending, divide by 100 before displaying.
- **GCash/Maya do not support auth holds**: capture immediately, refund on rejection. Cards support full auth/capture/void cycle.
- **Slot re-check on approval**: always re-validate slot availability inside the approval DB transaction. Another booking may have landed during the 24h window.
- **Webhook idempotency**: PayMongo retries webhooks on non-2xx. Always return 200 after logging; deduplicate by `event.ID` in `webhook_events`.
- **Redis cache invalidation**: when an owner publishes or updates a service/availability, actively invalidate all Geohash-5 cells within 10km of that branch — don't rely on TTL expiry alone.
- **Soft deletes everywhere**: never use `DELETE`. Set `deleted_at = now()`. All queries must include `WHERE deleted_at IS NULL`.
- **Notification delivery is async**: never send push notifications synchronously in a request handler. Always enqueue and let the worker deliver.
- **Review window CHECK constraint**: the DB enforces the 14-day window — do not duplicate this logic in the service layer.
- **Reconciliation job**: runs daily at 2am Manila time (`Asia/Manila`). Cross-checks `payment_intents` table against PayMongo API. Flags mismatches to `payment_reconciliation_alerts` table.

## Environment variables

See `.env.example` for all required variables. Key ones:

```
DATABASE_URL=postgres://...
REDIS_URL=redis://...
JWT_SECRET=...
PAYMONGO_SECRET_KEY=...
PAYMONGO_WEBHOOK_SECRET=...
FCM_SERVER_KEY=...
APNS_KEY_ID=...
APNS_TEAM_ID=...
APNS_BUNDLE_ID=...
S3_BUCKET=...
S3_REGION=ap-southeast-1
APP_ENV=development   # development | staging | production
```