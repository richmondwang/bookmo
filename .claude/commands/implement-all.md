# Implement all modules

Implement all backend modules for the booking platform, tracking progress in `docs/implementation-progress.md` so work can be resumed correctly across sessions.

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

### 0. auth (SSO)
Read `@docs/decisions/ADR-010-sso-authentication.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. Also add the `swagger:meta` block to `cmd/api/main.go` — this is the only module that does this.
Implement alongside the existing email/password auth:
- `internal/auth/sso.go` — VerifyGoogleToken, VerifyFacebookToken, VerifyAppleToken
- `internal/auth/repository.go` — GetIdentityByProvider, CreateIdentity, LinkIdentity, GetUserByEmail (already exists — extend)
- `internal/auth/service.go` — AuthenticateSSO (full flow: verify → lookup → new/returning/collision), StorePendingLink, VerifyPendingLink, SendOTP, VerifyOTP, SetPassword
- `internal/auth/handler.go` — POST /auth/sso, POST /auth/sso/verify-link, POST /auth/send-otp, POST /auth/verify-otp, PUT /auth/password
- `internal/auth/errors.go` — ErrEmailCollision (carries PendingLinkToken), ErrInvalidOTP, ErrOTPExpired, ErrOTPMaxAttempts, ErrInvalidProviderToken

Key rules:
- Never log or store raw provider tokens after verification
- OTP: bcrypt hash before Redis storage, max 3 attempts, delete key on max attempts
- POST /auth/send-otp always returns 200 — never leak email existence
- Apple: save apple_name.given_name + family_name on first sign-in only; absence on subsequent calls is expected
- pending_link_token: crypto/rand 32 bytes hex-encoded, 15-min Redis TTL
- email_verified = true for all SSO-created users

### 1. availability
Read `@docs/decisions/ADR-005-availability-resolution-priority.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement.
Implement:
- `internal/availability/model.go` — Slot, AvailabilityRule, DateOverride structs
- `internal/availability/repository.go` — GetDateOverride, GetAvailabilityRule, GetActiveBookingsInRange, GetActiveLocksInRange
- `internal/availability/service.go` — GetSlotsForDate (override → rule → closed), GenerateSlots, CheckConflict
- `internal/availability/handler.go` — GET /availability
- `internal/availability/errors.go` — ErrNoSlotsAvailable, ErrSlotConflict

### 2. bookings
Read `@docs/decisions/ADR-002-reschedule-keeps-original-confirmed.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. The Booking model must document all 7 status enum values.
Implement:
- `internal/bookings/model.go` — Booking (all 7 states), BookingLock, RescheduleRequest structs + DTOs
- `internal/bookings/repository.go` — Create, GetByID, UpdateStatus, CreateLock, DeleteLock, CreateRescheduleRequest, GetPendingRescheduleForBooking, ApproveReschedule (transaction), GetOwnerQueue
- `internal/bookings/service.go` — LockSlot, CreateBooking, Approve, Reject, Cancel, RequestReschedule, ApproveReschedule, RejectReschedule. State machine must reject illegal transitions.
- `internal/bookings/handler.go` — POST /bookings/lock, POST /bookings, GET /bookings, POST /bookings/:id/cancel, POST /bookings/:id/reschedule
- `internal/bookings/owner_handler.go` — GET /owner/queue, POST /owner/bookings/:id/approve, POST /owner/bookings/:id/reject, POST /owner/reschedules/:id/approve, POST /owner/reschedules/:id/reject
- `internal/bookings/errors.go` — ErrBookingNotFound, ErrSlotUnavailable, ErrIllegalStateTransition, ErrRescheduleLimitReached, ErrPendingRescheduleExists

### 3. payments
Read `@docs/decisions/ADR-001-payment-method-split.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. POST /payments/webhook has no Security: (public). All amount fields must note "in centavos".
Implement:
- `internal/payments/model.go` — PaymentIntent, Refund, WebhookEvent structs + PayMongoEvent DTO
- `internal/payments/repository.go` — CreatePaymentIntent, UpdatePaymentIntentStatus, CreateRefund, UpdateRefundStatus, IsWebhookEventProcessed, MarkWebhookEventProcessed
- `internal/payments/service.go` — CreateIntent (routes card vs e-wallet), HandleWebhook (verify sig, deduplicate, route by event type), CapturePayment, VoidPayment, RefundPayment
- `internal/payments/paymongo.go` — PayMongo API client (create intent, capture, void, refund, verify webhook signature)
- `internal/payments/handler.go` — POST /payments/intent, POST /payments/webhook (no JWT — public endpoint)
- `internal/payments/errors.go` — ErrPaymentNotFound, ErrInvalidWebhookSignature, ErrDuplicateWebhookEvent, ErrRefundFailed

### 4. notifications
Read `@docs/decisions/ADR-006-async-notifications.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement.
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


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. GET /search and GET /categories have no Security: (public).
Implement:
- `internal/search/model.go` — SearchParams, SearchResult structs
- `internal/search/repository.go` — SearchServices (PostGIS + FTS combined query with rating join), GetCategories
- `internal/search/cache.go` — GetFeedFromCache, SetFeedCache, InvalidateCellsNearBranch, feedCacheKey (Geohash-5)
- `internal/search/service.go` — Search (keyword → DB direct; no keyword → cache → DB fallback), GetSuggestions (Redis ZRANGEBYLEX), IncrementSuggestionScore
- `internal/search/handler.go` — GET /search, GET /categories, GET /search/suggestions
- `internal/search/errors.go` — ErrInvalidCoordinates

### 6. reviews
Read `@docs/decisions/ADR-003-rating-summaries-trigger.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. Document the 14-day window and the anonymous review rule (customer_id never returned when is_anonymous is true).
Implement:
- `internal/reviews/model.go` — Review, ReviewResponse, ReviewFlag, RatingSummary structs + DTOs. Anonymous reviews must never expose customer_id.
- `internal/reviews/repository.go` — Create, GetByService, CreateResponse, UpdateResponse, CreateFlag, GetFlagCount
- `internal/reviews/service.go` — Submit, Flag (auto-hide at 3 flags), RespondAsOwner. Never write to rating_summaries — it is trigger-owned.
- `internal/reviews/handler.go` — POST /reviews, GET /services/:id/reviews, GET /services/:id/reviews/summary, POST /reviews/:id/flag, POST /reviews/:id/response
- `internal/reviews/errors.go` — ErrReviewNotFound, ErrAlreadyReviewed, ErrBookingNotCompleted, ErrReviewWindowExpired, ErrNotBookingOwner

### 7. profiles
Read `@docs/decisions/ADR-007-dual-review-system-and-profiles.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. Document the role-aware response difference in GET /users/:id/profile.
Implement:
- `internal/profiles/model.go` — UserProfile, CustomerTrustProfile, ProfilePhotoUploadURL structs + role-aware response DTOs (OwnerViewProfile vs CustomerViewProfile)
- `internal/profiles/repository.go` — GetProfile, UpdateProfile, SetPhotoURL, GetTrustProfile
- `internal/profiles/service.go` — GetProfile (role-aware response), GeneratePhotoUploadURL (pre-signed S3), ConfirmPhotoUpload
- `internal/profiles/handler.go` — GET /users/:id/profile, PUT /users/me/profile, POST /users/me/photo/upload-url, POST /users/me/photo/confirm
- `internal/profiles/errors.go` — ErrProfileNotFound, ErrPhotoUploadFailed

### 8. customer_reviews
Read `@docs/decisions/ADR-007-dual-review-system-and-profiles.md` first (same ADR).


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement.
Implement:
- `internal/customer_reviews/model.go` — CustomerReview, CustomerReviewDispute structs + DTOs
- `internal/customer_reviews/repository.go` — Create, GetByCustomer, GetByBooking, CreateDispute
- `internal/customer_reviews/service.go` — Submit (validates completed booking, 14-day window, one per booking), Dispute. Never write to customer_trust_profiles — it is trigger-owned.
- `internal/customer_reviews/handler.go` — POST /customer-reviews (owner only), GET /users/:id/customer-reviews (owners only), POST /customer-reviews/:id/dispute (customer only)
- `internal/customer_reviews/errors.go` — ErrAlreadyReviewed, ErrBookingNotCompleted, ErrReviewWindowExpired, ErrDisputeAlreadyFiled, ErrNotYourReview

### 9. participants
Read `@docs/decisions/ADR-008-booking-participants.md` and `@docs/decisions/ADR-009-participant-eligibility.md` first.


> **Swagger**: Annotate every handler with `swagger:route` and every request/response struct with `swagger:model` as you implement. Document the 403 returned when service.allows_participants is false.
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


### 10. marketplace_payments
Read `@docs/decisions/ADR-012-marketplace-payments.md` first.

> **Swagger**: Annotate every handler with `swagger:route` and every struct with `swagger:model`. Document that all fee amounts use basis points. Note that admin endpoints require admin role.

Implement:
- `internal/marketplace/model.go` — OwnerPayoutAccount, OwnerEarning, OwnerPayout, BookingDispute, PlatformSettings, CategoryFeeRate, OwnerFeeOverride structs + DTOs
- `internal/marketplace/repository.go` — GetOwnerFeeOverride, GetCategoryFeeRate, GetPlatformSettings, CreateEarning, ReleaseEarning, CreateDispute, ResolveDispute, CreatePayoutAccount, GetDefaultPayoutAccount, VerifyPayoutAccount, CreatePayout, UpdatePayoutStatus, GetPendingEarnings, GetReleasedEarnings
- `internal/marketplace/fee.go` — ResolveFee (three-level priority, returns centavos + source string), always integer arithmetic with basis points
- `internal/marketplace/service.go` — RegisterPayoutAccount, SetDefaultAccount, VerifyAccountOTP (GCash/Maya), RaiseDispute, GetEarnings, GetPayouts, UpdatePayoutSchedule
- `internal/marketplace/scheduler.go` — ReleaseEarnings (finds expired dispute windows, creates owner_earnings rows), ProcessPayouts (batches released earnings per owner, initiates PayMongo transfer)
- `internal/marketplace/paymongo_payouts.go` — PayMongo Payouts API client (initiate transfer, check status)
- `internal/marketplace/handler.go` — owner endpoints: GET/POST/PUT/DELETE /owner/payout-accounts, POST /owner/payout-accounts/:id/verify-otp, GET /owner/earnings, GET /owner/payouts, PUT /owner/payout-schedule
- `internal/marketplace/customer_handler.go` — POST /bookings/:id/dispute
- `internal/marketplace/admin_handler.go` — admin endpoints: GET/POST /admin/disputes, POST /admin/disputes/:id/resolve, GET/POST /admin/payout-accounts/pending-verification, POST /admin/payout-accounts/:id/verify, POST/PUT /admin/fee-overrides, GET/PUT /admin/category-fee-rates, GET/PUT /admin/platform-settings
- `internal/marketplace/errors.go` — ErrNoVerifiedPayoutAccount, ErrDisputeWindowClosed, ErrAlreadyDisputed, ErrInvalidFeeConfig, ErrPayoutFailed

Key rules:
- ALWAYS use integer arithmetic for fee calculation: `gross * bp / 10000` — never float
- `net_amount_centavos` CHECK constraint will reject rows where net != gross - fee
- Set `bookings.completed_at` atomically when transitioning status to 'completed' in the bookings module
- One verified default account required before any payout can be initiated
- Disputes must be raised before `completed_at + dispute_window_hours` — reject with ErrDisputeWindowClosed after that
- Bank accounts require admin verification — GCash/Maya use OTP verification
- ReleaseEarnings scheduler: query `bookings WHERE status = 'completed' AND completed_at + (dispute_window_hours * interval '1 hour') < now() AND id NOT IN (SELECT booking_id FROM owner_earnings) AND id NOT IN (SELECT booking_id FROM booking_disputes WHERE status IN ('open','under_review'))`

## Step 4 — After each module

After completing each module:

1. Run `go build ./...` — fix all compile errors before moving on
2. Run `go test -race ./internal/<module>/...` — fix any failures
3. Run `swagger generate spec -o docs/openapi.yaml --scan-models && swagger validate docs/openapi.yaml` — fix any annotation errors before moving on
4. **Update `docs/implementation-progress.md`** with:
   - Mark the module as `done` in the checklist table
   - Add a row to "Completed modules" with: module name, timestamp, any notable decisions or deviations from the ADR
   - Update "Last session notes" with what was just completed and what is next
   - Set the overall Status line to `IN PROGRESS` (or `COMPLETE` if all done)

The progress file update must happen after every module — not at the end of all modules.

## Step 5 — Final step

When all 9 modules are marked `done` and the swagger spec is valid (it should be, since you validated after each module):

1. Run `go build ./...` — full project must compile cleanly
2. Run `go test -race ./...` — all tests must pass
3. Update `docs/implementation-progress.md` Status to `COMPLETE`
4. Do a final full swagger regeneration: `swagger generate spec -o docs/openapi.yaml --scan-models && swagger validate docs/openapi.yaml`
5. Print a summary of all modules implemented and any decisions that deviated from the ADRs

## Rules that apply throughout

- **Every handler must have `swagger:route` at time of writing** — not afterward, not as a separate step. A handler without this annotation is not done.
- **Every model struct used in a request or response must have `swagger:model`** — add it when you write the struct, not later.
- **After each module, run `swagger generate spec -o docs/openapi.yaml --scan-models && swagger validate docs/openapi.yaml`** — fix any annotation errors before moving to the next module.
- Never skip the progress file update after a module — it is how sessions resume correctly
- Never implement a module out of order — dependency order exists for a reason
- Soft deletes everywhere: all queries filter `WHERE deleted_at IS NULL`
- Money in centavos (int) always — never float
- Errors wrapped with context: `fmt.Errorf("module.Function: %w", err)`
- Push notifications enqueued, never sent synchronously
- Never write to `rating_summaries` or `customer_trust_profiles` from application code
- Never log or retain raw SSO provider tokens after extracting claims
- POST /auth/send-otp must always return 200 — never reveal email existence
- Expose resolved `allows_participants: bool` on GET /services/:id — compute from the three-level resolution, not the raw nullable