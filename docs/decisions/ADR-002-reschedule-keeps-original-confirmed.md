# ADR-002: Reschedule requests keep the original booking confirmed

## Status
Accepted

## Context

When a customer wants to reschedule a confirmed booking, there are two possible approaches:

**Option A — Optimistic:** Move the original booking to a `pending_reschedule` state immediately when the customer submits the request. If the owner rejects the new time, restore the original booking to `confirmed`.

**Option B — Conservative:** Keep the original booking `confirmed` throughout. Only transition it to `rescheduled` when the owner explicitly approves the new time.

## Decision

Use Option B — the original booking stays `confirmed` until the owner approves the reschedule.

## Reasoning

Option A creates a gap where the customer has no confirmed booking. If the owner rejects the new time or simply doesn't respond, restoring the original booking requires extra state management and introduces the risk of the slot being taken by another customer in the interim.

In practice, owners in the Philippines market are small business operators who may take hours to respond to a reschedule request. A customer whose confirmed booking disappears while they wait for owner approval will perceive this as the platform losing their booking — a significant trust issue.

Option B means the customer always has a confirmed booking. If the reschedule is rejected, nothing changes — they still have their original slot. If approved, the old booking moves to `rescheduled` and a new confirmed booking is created atomically.

## Consequences

- `reschedule_requests` is a separate table from `bookings` — it is a proposal, not a booking state
- The original booking's `status` field is never set to any intermediate state during a pending reschedule
- The owner sees the reschedule request in their approval queue alongside new bookings
- On approval, the service layer must run a single DB transaction that:
  1. Re-checks slot availability for the new time
  2. Sets `reschedule_requests.status = 'approved'`
  3. Sets original `bookings.status = 'rescheduled'`
  4. Inserts a new booking with `status = 'confirmed'` and `rescheduled_from_booking_id` pointing to the original
  5. Auto-confirms the new booking — no second approval cycle required since the owner is already actively approving
- If the new slot is taken between when the customer proposed it and when the owner approves, the transaction must return a conflict error and leave both the original booking and the reschedule request unchanged
- Maximum 3 reschedule attempts per booking (`reschedule_attempt_count` on the `bookings` row, enforced before creating a new `reschedule_requests` row)
- Only one pending reschedule request per booking at a time, enforced by a partial unique index:
  ```sql
  CREATE UNIQUE INDEX idx_one_pending_reschedule
    ON reschedule_requests (booking_id)
    WHERE status = 'pending';
  ```
