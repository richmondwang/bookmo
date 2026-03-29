# ADR-008: Booking participants — tracking who booked together

## Status
Accepted

## Context

Customers need a way to tag other registered users on a booking for record-keeping purposes. This enables future features like "people you've booked with", social discovery, and shared booking history. This feature intentionally has no effect on payment, booking state, reviews, or owner approval.

## Decisions

### Scope is purely recording
Participants are a social layer on top of an existing booking. They do not:
- Affect the booking state machine
- Create additional payment intents
- Generate review prompts
- Appear in the owner's approval queue
- Block or delay any booking flow

### When participants can be added
Any time — before or after owner approval, before or after confirmation. The only boundary is completion: once a booking reaches `completed` status, no new participants can be added and existing ones cannot leave. This is enforced at the service layer.

### Acceptance required
A tagged user receives a push notification and must explicitly accept or decline. This prevents unwanted tagging and keeps the "booked with" history meaningful — only mutually agreed participation is recorded. Pending and declined participants are not included in any social history queries.

### Leaving
An accepted participant can leave at any time before the booking is `completed`. Once completed, the record is permanent. `left_at` is set on leave; the row is never deleted.

### Who can add participants
Only the booking creator (`bookings.customer_id`). Participants cannot invite other participants.

### No capacity enforcement
Participants are not counted against `services.capacity`. Capacity is a booking-level concept — the booking already occupies its slot. Participants are just people tagged on that booking.

## Consequences

### New table
```sql
booking_participants (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id    UUID NOT NULL REFERENCES bookings(id),
  user_id       UUID NOT NULL REFERENCES users(id),
  invited_by    UUID NOT NULL REFERENCES users(id),
  status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','accepted','declined','left')),
  invited_at    TIMESTAMP NOT NULL DEFAULT now(),
  responded_at  TIMESTAMP,
  left_at       TIMESTAMP,
  UNIQUE (booking_id, user_id)
)
```

### Two new notification types
- `participant_invited` — sent to the invited user when tagged
- `participant_accepted` — sent to the booking creator when someone accepts

### "Booked with" query
Only rows with `status = 'accepted'` and `left_at IS NULL` count. Future social features query:
```sql
SELECT DISTINCT bp.user_id
FROM booking_participants bp
JOIN bookings b ON b.id = bp.booking_id
WHERE b.customer_id = $1        -- bookings created by this user
  AND bp.status = 'accepted'
  AND bp.left_at IS NULL
  AND b.status = 'completed';
```

Or the inverse — bookings where this user was a participant:
```sql
SELECT DISTINCT b.customer_id
FROM booking_participants bp
JOIN bookings b ON b.id = bp.booking_id
WHERE bp.user_id = $1
  AND bp.status = 'accepted'
  AND bp.left_at IS NULL
  AND b.status = 'completed';
```

### Service layer constraints
- Creator cannot invite themselves (`user_id = bookings.customer_id` → `ErrCannotInviteSelf`)
- Cannot invite the same user twice (`UNIQUE` constraint → `ErrAlreadyInvited`)
- Cannot add participants once booking is `completed` → `ErrBookingCompleted`
- Cannot leave once booking is `completed` → `ErrBookingCompleted`
- Only the booking creator can invite → `ErrNotBookingCreator`
- Only the invited user can accept/decline/leave → `ErrNotParticipant`
