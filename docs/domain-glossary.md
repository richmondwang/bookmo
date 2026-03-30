## Authentication

**SSO (Single Sign-On)**
Authentication via a third-party provider (Google, Facebook, Apple) instead of email + password. The mobile app obtains an identity token from the provider's SDK and sends it to `POST /auth/sso`. The backend verifies the token with the provider and issues the app's own JWT.

**User identity**
A row in `user_identities` linking a user account to a specific SSO provider. One user can have multiple identities (e.g. linked to both Google and Facebook). Each identity has a `provider` and `provider_id` (the unique ID the provider uses for that user).

**Provider ID**
The unique identifier a provider assigns to a user — Google's `sub`, Facebook's `id`, Apple's `sub`. Stored in `user_identities.provider_id`. Never changes for a given user + provider combination.

**Pending link token**
A short-lived Redis key (15-minute TTL) created when a new SSO provider's email matches an existing account. The token stores the `provider` and `provider_id` waiting to be linked after the user verifies ownership of the existing account. Never stored in the database.

**Email collision**
When a user signs in with a new SSO provider and the email from that provider already exists in `users`. Handled by returning HTTP 409 with a `pending_link_token`. Requires verification before linking — never merged silently.

**OTP (One-Time Password)**
A 6-digit code sent to a user's email to verify ownership. Used in the email collision merge flow. Hashed with bcrypt before storing in Redis. Max 3 attempts; TTL 10 minutes.

**email_verified**
A boolean on `users`. SSO users are `true` on creation (provider verified it). Email/password users are `false` until they complete OTP verification.

# Domain glossary

Precise definitions for all domain terms used in this codebase. When in doubt about naming a variable, function, or column — use these terms exactly.

---

## Core entities

**User**
A person with an account. Has one of three roles: `customer`, `owner`, or `admin`. A user who is an owner also has an `Owner` record linked to them.

**Owner**
A business operator who lists services. One owner can have multiple branches. An owner is always backed by a `User` record with `role = 'owner'`. Do not confuse `owner_id` (references the `owners` table) with `user_id` (references the `users` table).

**Branch**
A physical location belonging to an owner. Services are listed under a branch, not directly under an owner. Availability is set per branch. A single owner can operate multiple branches (e.g. a chain with locations in Makati and BGC).

**Service**
A bookable offering listed under a branch. Has a name, duration range, capacity, and price. Examples: "Tennis Court Rental", "Full Body Massage 60min", "Meeting Room A". A branch can have multiple services.

**Category**
A taxonomy label for services. Hierarchical — categories can have a `parent_id`. Used for filtering in search. Examples: "Sports & Fitness" → "Tennis Court". Stored in the `categories` table, referenced by `services.category_id`.

---

## Availability

**Slot**
A specific bookable time window for a service on a given day. Slots are computed at query time by the availability engine — they are not stored in the database. A slot has a `start_time`, `end_time`, and `remaining_capacity`.

**Availability rule**
A recurring weekly schedule entry for a branch. Defines which days of the week the branch is open and during what hours. Stored in `availability_rules`. Applies to all services under the branch unless overridden.

**Date override**
A one-off exception to the weekly availability rules for a specific calendar date. Can mark a branch as fully closed (`is_closed = true`) or set custom hours for that day. Stored in `date_overrides`. Always takes priority over `availability_rules` — see ADR-005.

**Capacity**
The maximum number of simultaneous bookings a service slot can hold. A tennis court with `capacity = 1` can only have one booking per slot. A group fitness class with `capacity = 20` allows up to 20 bookings in the same slot.

**Capacity type**
Either `single` (at most 1 booking per slot, reject any overlap) or `multi` (up to `capacity` bookings per slot, sum of quantities must not exceed capacity).

---

## Booking lifecycle

**Lock**
A short-lived reservation of a slot, created before payment is initiated. Prevents other customers from booking the same slot while the current customer is completing payment. Stored in `booking_locks`. TTL: 2–5 minutes. Deleted when the booking is confirmed or when it expires. A lock is not a booking.

**Booking**
A confirmed intent by a customer to use a service at a specific time. Created after payment is initiated (or completed for e-wallets). Progresses through the state machine. The authoritative record of a reservation.

**Booking state machine**
The 7 states a booking can be in, and the valid transitions between them:
- `pending` — created after payment intent initiated, awaiting payment confirmation
- `awaiting_approval` — payment confirmed, waiting for owner to approve within 24h
- `confirmed` — owner approved; the booking is active
- `rejected` — owner declined; payment voided or refunded
- `cancelled` — cancelled by customer, owner, or system after confirmation; refund may apply
- `rescheduled` — this booking was replaced by a new booking via a reschedule approval; treated as terminal
- `completed` — the service date has passed; the booking is closed

**Owner response deadline**
The timestamp by which an owner must approve or reject a booking in `awaiting_approval` status. Set to 24 hours after the booking is created. If the deadline passes without a response, the system auto-cancels and refunds. Stored as `bookings.owner_response_deadline`.

**Reschedule request**
A customer's proposal to move a confirmed booking to a new time. Stored in `reschedule_requests`. The original booking stays `confirmed` while a reschedule request is `pending`. Not a booking — it is a proposal. At most one pending reschedule request per booking at any time.

---

## Payments

**Payment intent**
A record tracking a single payment attempt through PayMongo. Created when the customer initiates payment. Contains the PayMongo payment intent ID, amount in centavos, method, and status. Stored in `payment_intents`. One payment intent per booking.

**Authorize**
The first step of a card payment. The bank reserves funds on the customer's card without transferring them. The customer is not charged yet. Used for cards only — GCash and Maya do not support this.

**Capture**
The second step of a card payment. Funds are transferred from the customer's card to the merchant. Triggered when the owner approves the booking.

**Void**
Releasing an authorization hold on a card without capturing. The customer is never charged. Used when a booking is rejected or times out. Cards only — GCash/Maya payments cannot be voided, only refunded.

**Refund**
Returning funds to a customer after they have already been captured. Used for GCash/Maya rejections and cancellations. Takes 3–7 banking days to appear in the customer's wallet. Tracked in the `refunds` table.

**Centavos**
The unit for all monetary amounts in the system. ₱1 = 100 centavos. All amounts stored in the database and sent to PayMongo are in centavos as integers. Never use floats for money. Display conversion (÷100) happens only in response DTOs.

---

## Notifications

**Push notification**
A message sent to a user's device via FCM (Android) or APNs (iOS). Delivery is always async — never sent inside a request handler. Tracked in `notification_logs`.

**In-app notification**
A persistent notification stored in the `notifications` table and surfaced in the app's notification feed (bell icon). Written synchronously as a DB row. Separate from push notifications, though both are usually created together for the same event.

**Device token**
A unique identifier for a specific user's device, provided by FCM or APNs. Required to send push notifications. Stored in `device_tokens`. A user can have multiple active tokens (multiple devices). Tokens marked `is_active = false` are never used for delivery.

---

## Reviews

**Review**
A customer's rating and optional written feedback for a service, submitted after a completed booking. One review per completed booking (enforced by a UNIQUE constraint on `booking_id`). Must be submitted within 14 days of the booking's `end_time`.

**Review window**
The 14-day period after a booking's `end_time` during which the customer can submit a review. Enforced by a CHECK constraint on the `reviews` table. Not duplicated in application code.

**Rating summary**
A pre-aggregated row in `rating_summaries` storing the total review count, total rating sum, average rating, and per-star counts for a service. Updated by a database trigger — never written to directly by application code. Used for fast rating display in search results.

**Review response**
An owner's public reply to a customer's review. One response per review (enforced by UNIQUE on `review_id`). Cannot be used to remove or edit the review.

---

## Profiles and customer trust

**Customer profile**
A user's public identity. Contains `full_name`, `profile_photo_url`, `bio`, `is_verified`, and `joined_at`. Profile photos are stored in S3 — `profile_photo_url` is the CDN URL, never an S3 key or upload URL. Visibility is role-aware: owners see the full trust profile; other customers see only basic fields.

**Profile photo upload**
A two-step pre-signed URL flow. Never stream file bytes through the API server. Step 1: `POST /users/me/photo/upload-url` returns a pre-signed S3 upload URL and the final CDN URL. Step 2: mobile app uploads directly to S3. Step 3: `POST /users/me/photo/confirm` sets `users.profile_photo_url` to the CDN URL.

**Customer trust profile**
A pre-aggregated row in `customer_trust_profiles` storing reliability signals: `total_bookings`, `completed_bookings`, `cancelled_bookings`, `completion_rate`, `cancellation_rate`, `avg_owner_rating`, `total_owner_reviews`. Updated by trigger — never by application code. Visible to all owners; not to other customers.

**Completion rate**
`completed_bookings / total_bookings * 100`, rounded to one decimal. Denominator counts only bookings that reached `confirmed`, `completed`, or `cancelled` — not `pending` or `awaiting_approval`.

**Customer review**
An owner's rating of a customer after a completed booking. Stored in `customer_reviews`. One per completed booking (UNIQUE on `booking_id`). 14-day window. Visible to all owners. Customers can see their own. Other customers cannot.

**Customer review dispute**
A customer's formal contest of an owner review. Stored in `customer_review_disputes`. One per review per customer. Does not hide the review — only admin resolution can remove it. Reasons: `inaccurate`, `inappropriate`, `not_my_booking`, `retaliation`.

---

## Booking participants

**Booking participant**
A registered user tagged on a booking by the booking creator for record-keeping purposes. Stored in `booking_participants`. Has no effect on payment, booking state, reviews, or owner approval. Used to build "booked with" history for future social features.

**Participant status**
One of four values: `pending` (invited, awaiting response), `accepted` (confirmed participation), `declined` (explicitly rejected the invitation), `left` (accepted but later removed themselves). Only `accepted` rows with `left_at IS NULL` count toward "booked with" history.

**Invited by**
Always the booking creator (`bookings.customer_id`). Participants cannot invite other participants. Stored as `booking_participants.invited_by` to support future co-organizer features without a schema change.

**Participant eligibility**
Whether a service allows participants to be tagged on its bookings. Resolved at invite time from three sources in priority order: `services.allows_participants` (owner override) → `categories.allows_participants` (category default) → parent category `allows_participants` → `false` (system default). `NULL` on any source means "not set — check next source". The resolved boolean is exposed as `allows_participants` on service API responses.

**Booked with**
A derived social relationship between two users who have at least one completed booking where one created it and the other accepted a participant invitation. Queried from `booking_participants` joined to `bookings` — never stored as a separate table.

---

## Search

**Feed**
The ranked list of services shown on the customer's home screen when no keyword is entered. Results are ranked by distance and rating. Cached in Redis by Geohash grid cell.

**Geohash cell**
A geographic grid cell used as a Redis cache key for the feed. Uses Geohash precision-5 (approximately 4.9km × 4.9km). All customers within the same cell receive the same cached feed result.

**FTS (full-text search)**
PostgreSQL's built-in full-text search, used for keyword queries. Powered by `tsvector` columns on the `services` table, updated by trigger. Used when the customer types a search term — bypasses the feed cache entirely.