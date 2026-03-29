# ADR-007: Dual review system — owners review customers, customers have trust profiles

## Status
Accepted

## Context

The original review system only went one direction: customers review services. This ADR covers two new features added together because they are tightly coupled:

1. **Owner reviews of customers** — after a completed booking, the owner can rate the customer (1–5 stars, optional body text). Any owner can see these reviews before approving a booking.
2. **Customer trust profiles** — a pre-aggregated summary of a customer's reliability: completion rate, cancellation rate, average owner rating. Visible to all owners.

## Decisions

### Who can see owner-written reviews of a customer
All owners can see them — not just owners the customer has previously booked with. This makes the trust network meaningful: an owner in Makati benefits from a review written by an owner in BGC. Reviews are surfaced on the customer's profile and in the approval queue card.

### Can customers see reviews owners wrote about them
Yes, always visible. Customers can also dispute or flag a review they believe is unfair, inaccurate, or retaliatory. A dispute does not hide the review — it queues it for admin moderation. The customer gets one dispute per review.

### Profile visibility
Owners always see the full trust profile including owner reviews. Other customers see only the basic profile (name, photo, join date) — no trust signals, no owner reviews. This is enforced at the API layer by checking the caller's role.

### Profile photos
Stored in S3 via pre-signed URL upload — the API server never handles the file bytes. Two-step flow:
1. Client calls `POST /users/me/photo/upload-url` → receives a pre-signed S3 URL and the final CDN URL
2. Client uploads directly to S3
3. Client calls `POST /users/me/photo/confirm` → server sets `users.profile_photo_url`

Never store the upload URL in the database — only the final CDN URL.

## Consequences

### New tables
- `customer_reviews` — owner-written reviews of customers. UNIQUE on `booking_id` (one per completed booking, same as service reviews).
- `customer_review_disputes` — customer contest records. UNIQUE on `(customer_review_id, customer_id)` (one dispute per review).
- `customer_trust_profiles` — pre-aggregated trust signals per customer. Updated by trigger, never by application code.

### Users table additions
```sql
ALTER TABLE users ADD COLUMN full_name          TEXT;
ALTER TABLE users ADD COLUMN bio                TEXT CHECK (char_length(bio) <= 300);
ALTER TABLE users ADD COLUMN profile_photo_url  TEXT;
ALTER TABLE users ADD COLUMN is_verified        BOOLEAN NOT NULL DEFAULT false;
```

### Trigger ownership
`customer_trust_profiles` is updated by two triggers:
- One on `bookings` (fires when `status` changes to `completed` or `cancelled`) — updates `total_bookings`, `completed_bookings`, `cancelled_bookings`, `completion_rate`, `cancellation_rate`
- One on `customer_reviews` (fires on INSERT or when `status` changes to `removed`) — updates `avg_owner_rating`, `total_owner_reviews`

Never write to `customer_trust_profiles` from application code. Never read from `bookings` or `customer_reviews` at query time to compute trust signals — always read from `customer_trust_profiles`.

### Role-aware profile response
The `GET /users/:id/profile` endpoint must check the caller's role:
- `owner` role → return full profile including `trust` object and `owner_reviews` array
- `customer` role viewing another customer → return basic profile only (name, photo, join date)
- `customer` role viewing their own profile → return full profile of their own data

This check must happen in the service layer, not the handler, so it can be tested independently.

### Approval queue enrichment
The owner approval queue response (`GET /owner/queue`) must JOIN `customer_trust_profiles` and include a `customer_trust` object on each queue item. Owners see this before approving, which is the primary value of the feature.

### Owner review timing
Owner can submit a review after `booking.status = 'completed'`. Same 14-day window as customer reviews, enforced by CHECK constraint. One review per completed booking (UNIQUE on `booking_id`).

### Dispute resolution
A dispute sets `customer_review_disputes.status = 'open'` and notifies admins. The review remains published during review. Admin resolves by either:
- Setting `customer_review_disputes.status = 'resolved'` and leaving the review published
- Setting `customer_reviews.status = 'removed'` — trigger then subtracts from `customer_trust_profiles`
