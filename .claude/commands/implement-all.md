# Implement all modules

Implement all backend modules for the BookMo booking platform, tracking progress in `docs/implementation-progress.md` so work can be resumed correctly across sessions.

## Step 1 — Read context

Before doing anything else, read these files:
- `@CLAUDE.md` — architecture, conventions, critical rules
- `@docs/domain-glossary.md` — precise naming for all domain terms
- `@docs/implementation-progress.md` — current progress state

## Step 2 — Determine where to start

Check `docs/implementation-progress.md`:
- Any module marked `in_progress` — resume it first, it was interrupted
- Any module marked `pending` — implement in the order listed below
- Any module marked `done` — skip it entirely, do not re-implement

## Step 3 — Implementation order

Implement modules in this exact order (dependency order — each module may import the ones above it):

### 1. availability
Read `@docs/decisions/ADR-005-availability-resolution-priority.md` first.

Implement:
- `internal/availability/model.go` — Slot, AvailabilityRule, DateOverride structs
- `internal/availability/repository.go` — GetDateOverride, GetAvailabilityRule, GetActiveBookingsInRange, GetActiveLocksInRange
- `internal/availability/service.go` — GetSlotsForDate (override → rule → closed), GenerateSlots, CheckConflict
- `internal/availability/handler.go` — GET /availability
- `internal/availability/errors.go` — ErrNoSlotsAvailable, ErrSlotConflict

### 2. bookings
Read `@docs/decisions/ADR-002-reschedule-keeps-original-confirmed.md` first.

Implement:
- `internal/bookings/model.go` — Booking (all 7 states), BookingLock, RescheduleRequest structs + DTOs
- `internal/bookings/repository.go` — Create, GetByID, UpdateStatus, CreateLock, DeleteLock, CreateRescheduleRequest, GetPendingRescheduleForBooking, ApproveReschedule (transaction), GetOwnerQueue
- `internal/bookings/service.go` — LockSlot, CreateBooking, Approve, Reject, Cancel, RequestReschedule, ApproveReschedule, RejectReschedule. State machine must reject illegal transitions.
- `internal/bookings/handler.go` — POST /bookings/lock, POST /bookings, GET /bookings, POST /bookings/:id/cancel, POST /bookings/:id/reschedule
- `internal/bookings/owner_handler.go` — GET /owner/queue, POST /owner/bookings/:id/approve, POST /owner/bookings/:id/reject, POST /owner/reschedules/:id/approve, POST /owner/reschedules/:id/reject
- `internal/bookings/errors.go` — ErrBookingNotFound, ErrSlotUnavailable, ErrIllegalStateTransition, ErrRescheduleLimitReached, ErrPendingRescheduleExists

### 3. payments
Read `@docs/decisions/ADR-001-payment-method-split.md` first.

Implement:
- `internal/payments/model.go` — PaymentIntent, Refund, WebhookEvent structs + PayMongoEvent DTO
- `internal/payments/repository.go` — CreatePaymentIntent, UpdatePaymentIntentStatus, CreateRefund, UpdateRefundStatus, IsWebhookEventProcessed, MarkWebhookEventProcessed
- `internal/payments/service.go` — CreateIntent (routes card vs e-wallet), HandleWebhook (verify sig, deduplicate, route by event type), CapturePayment, VoidPayment, RefundPayment
- `internal/payments/paymongo.go` — PayMongo API client (create intent, capture, void, refund, verify webhook signature)
- `internal/payments/handler.go` — POST /payments/intent, POST /payments/webhook (no JWT — public endpoint)
- `internal/payments/errors.go` — ErrPaymentNotFound, ErrInvalidWebhookSignature, ErrDuplicateWebhookEvent, ErrRefundFailed

### 4. notifications
Read `@docs/decisions/ADR-006-async-notifications.md` first.

Implement:
- `internal/notifications/model.go` — Notification, DeviceToken, NotificationLog, NotificationJob structs
- `internal/notifications/repository.go` — CreateNotification, GetUnread, MarkRead, GetActiveDeviceTokens, DeactivateToken, LogDeliveryAttempt, SaveDeviceToken
- `internal/notifications/service.go` — Enqueue (writes job to Redis list), CreateInAppNotification (synchronous DB write)
- `internal/notifications/worker.go` — Consume (BRPOP loop), Deliver (FCM + APNs), handleInvalidToken, retryWithBackoff
- `internal/notifications/fcm.go` — FCM HTTP v1 API client
- `internal/notifications/apns.go` — APNs HTTP/2 client
- `internal/notifications/handler.go` — GET /notifications, POST /notifications/device-token
- `internal/notifications/errors.go` — ErrInvalidToken, ErrDeliveryFailed

### 5. search
Read `@docs/decisions/ADR-004-geohash-feed-cache.md` first.

Implement:
- `internal/search/model.go` — SearchParams, SearchResult structs
- `internal/search/repository.go` — SearchServices (PostGIS + FTS combined query with rating join), GetCategories
- `internal/search/cache.go` — GetFeedFromCache, SetFeedCache, InvalidateCellsNearBranch, feedCacheKey (Geohash-5)
- `internal/search/service.go` — Search (keyword → DB direct; no keyword → cache → DB fallback), GetSuggestions (Redis ZRANGEBYLEX), IncrementSuggestionScore
- `internal/search/handler.go` — GET /search, GET /categories, GET /search/suggestions
- `internal/search/errors.go` — ErrInvalidCoordinates

### 6. reviews
Read `@docs/decisions/ADR-003-rating-summaries-trigger.md` first.

Implement:
- `internal/reviews/model.go` — Review, ReviewResponse, ReviewFlag, RatingSummary structs + DTOs. Anonymous reviews must never expose customer_id.
- `internal/reviews/repository.go` — Create, GetByService, CreateResponse, UpdateResponse, CreateFlag, GetFlagCount
- `internal/reviews/service.go` — Submit, Flag (auto-hide at 3 flags), RespondAsOwner. Never write to rating_summaries — it is trigger-owned.
- `internal/reviews/handler.go` — POST /reviews, GET /services/:id/reviews, GET /services/:id/reviews/summary, POST /reviews/:id/flag, POST /reviews/:id/response
- `internal/reviews/errors.go` — ErrReviewNotFound, ErrAlreadyReviewed, ErrBookingNotCompleted, ErrReviewWindowExpired, ErrNotBookingOwner

### 7. profiles
Read `@docs/decisions/ADR-007-dual-review-system-and-profiles.md` first.

Implement:
- `internal/profiles/model.go` — UserProfile, CustomerTrustProfile, ProfilePhotoUploadURL structs + role-aware response DTOs (OwnerViewProfile vs CustomerViewProfile)
- `internal/profiles/repository.go` — GetProfile, UpdateProfile, SetPhotoURL, GetTrustProfile
- `internal/profiles/service.go` — GetProfile (role-aware response), GeneratePhotoUploadURL (pre-signed S3), ConfirmPhotoUpload
- `internal/profiles/handler.go` — GET /users/:id/profile, PUT /users/me/profile, POST /users/me/photo/upload-url, POST /users/me/photo/confirm
- `internal/profiles/errors.go` — ErrProfileNotFound, ErrPhotoUploadFailed

### 8. customer_reviews
Read `@docs/decisions/ADR-007-dual-review-system-and-profiles.md` first (same ADR).

Implement:
- `internal/customer_reviews/model.go` — CustomerReview, CustomerReviewDispute structs + DTOs
- `internal/customer_reviews/repository.go` — Create, GetByCustomer, GetByBooking, CreateDispute
- `internal/customer_reviews/service.go` — Submit (validates completed booking, 14-day window, one per booking), Dispute. Never write to customer_trust_profiles — it is trigger-owned.
- `internal/customer_reviews/handler.go` — POST /customer-reviews (owner only), GET /users/:id/customer-reviews (owners only), POST /customer-reviews/:id/dispute (customer only)
- `internal/customer_reviews/errors.go` — ErrAlreadyReviewed, ErrBookingNotCompleted, ErrReviewWindowExpired, ErrDisputeAlreadyFiled, ErrNotYourReview

### 9. participants
Read `@docs/decisions/ADR-008-booking-participants.md` and `@docs/decisions/ADR-009-participant-eligibility.md` first.

Implement:
- `internal/participants/model.go` — BookingParticipant struct, ParticipantStatus type, request/response DTOs
- `internal/participants/repository.go` — Invite, GetByBooking, GetByUser, Accept, Decline, Leave, GetBookedWith
- `internal/participants/service.go` — Invite (validate creator, not self, not completed, not duplicate), Accept, Decline, Leave (validate not completed), GetBookedWith
- `internal/participants/handler.go` — POST /bookings/:id/participants, GET /bookings/:id/participants, POST /bookings/:id/participants/:user_id/accept, POST /bookings/:id/participants/:user_id/decline, DELETE /bookings/:id/participants/me, GET /users/:id/booked-with
- `internal/participants/errors.go` — ErrNotBookingCreator, ErrCannotInviteSelf, ErrAlreadyInvited, ErrBookingCompleted, ErrNotParticipant, ErrParticipantsNotAllowed

Key rules:
- Resolve eligibility FIRST in Invite — call repo.ResolveParticipantEligibility before any other check. Return ErrParticipantsNotAllowed (HTTP 403) if false.
- Eligibility query joins booking → service → category → parent category; resolves service override → category → parent → false
- Never touch booking state, payment, or review logic from this module
- Enqueue participant_invited and participant_accepted notifications via notification service — never send synchronously
- GetBookedWith returns users from BOTH directions: bookings the user created where others accepted, AND bookings where this user accepted

## Step 4 — After each module

After completing each module:

1. Run `go build ./...` — fix all compile errors before moving on
2. Run `go test -race ./internal/<module>/...` — fix any failures
3. **Update `docs/implementation-progress.md`** with:
   - Mark the module as `done` in the checklist table
   - Add a row to "Completed modules" with: module name, timestamp, any notable decisions or deviations from the ADR
   - Update "Last session notes" with what was just completed and what is next
   - Set the overall Status line to `IN PROGRESS` (or `COMPLETE` if all done)

The progress file update must happen after every module — not at the end of all modules.

## Step 5 — Final step

When all 9 modules are marked `done`:

1. Run `go build ./...` — full project must compile cleanly
2. Run `go test -race ./...` — all tests must pass
3. Update `docs/implementation-progress.md` Status to `COMPLETE`
4. Print a summary of all modules implemented and any decisions that deviated from the ADRs

## Rules that apply throughout

- Never skip the progress file update after a module — it is how sessions resume correctly
- Never implement a module out of order — dependency order exists for a reason
- If a compile error in module B is caused by module A, fix A first
- Soft deletes everywhere: all queries filter `WHERE deleted_at IS NULL`
- Money in centavos (int) always — never float
- Errors wrapped with context: `fmt.Errorf("module.Function: %w", err)`
- Push notifications enqueued, never sent synchronously
- Never write to `rating_summaries` or `customer_trust_profiles` from application code
- Expose resolved `allows_participants: bool` on GET /services/:id — compute from the three-level resolution, not the raw nullable