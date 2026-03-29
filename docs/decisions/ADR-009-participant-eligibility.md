# ADR-009: Participant eligibility — category default with service override

## Status
Accepted

## Context

Booking participants are only meaningful for certain types of services — a basketball court booking makes sense to share with friends, but a one-on-one haircut appointment does not. Eligibility needs to be controlled at the category level (as a sensible default) with the ability for owners to override it per service.

## Decisions

### Where eligibility lives
Two nullable boolean columns — one on `categories`, one on `services`:
- `categories.allows_participants BOOLEAN DEFAULT NULL`
- `services.allows_participants BOOLEAN DEFAULT NULL`

`NULL` always means "not explicitly set — look elsewhere". It is never treated as `false`.

### Resolution priority (highest to lowest)
1. `services.allows_participants` — if the owner has explicitly set it, this wins
2. `categories.allows_participants` — the direct category of the service
3. `categories.allows_participants` of the parent category — walked up one level
4. `false` — the system default if nothing is set anywhere in the chain

The tree walk stops at depth 2 (direct category + one parent). Walking the full tree to arbitrary depth adds complexity for minimal real-world gain — categories in this domain are at most 2 levels deep (e.g. "Sports & Fitness" → "Basketball Court").

### API behaviour
The invite endpoint (`POST /bookings/:id/participants`) resolves eligibility on every call. If the resolved value is `false`, the API returns `ErrParticipantsNotAllowed` (HTTP 403). The UI hides the invite button when it knows the service is ineligible, but the API enforces it regardless — the UI check is a convenience, not the gate.

### Owner UX
When an owner creates or edits a service, the UI shows the inherited default from the category alongside a toggle to override. If the owner doesn't touch the toggle, `services.allows_participants` stays `NULL` and the category default applies automatically. If the category default later changes (e.g. an admin enables participants for a whole category), all services that haven't overridden will inherit the new default immediately — no migration needed.

### Admin control
Platform admins set `categories.allows_participants` for each category. This is a one-time seed for the Philippines market category taxonomy. Suggested defaults:

| Category | Default |
|---|---|
| Sports & Fitness | `true` |
| Recreation (billiards, bowling, karaoke) | `true` |
| Events & Venues | `true` |
| Beauty & Wellness | `false` |
| Services (tutoring, lessons) | `false` |

## Consequences

### Schema
```sql
ALTER TABLE categories ADD COLUMN allows_participants BOOLEAN DEFAULT NULL;
ALTER TABLE services   ADD COLUMN allows_participants BOOLEAN DEFAULT NULL;
```

### Resolution function in Go
```go
// ResolveAllowsParticipants resolves the effective eligibility for a service.
// Joins are done in the eligibility query — this function operates on the result.
func ResolveAllowsParticipants(
    serviceOverride *bool,
    categoryValue   *bool,
    parentValue     *bool,
) bool {
    if serviceOverride != nil {
        return *serviceOverride
    }
    if categoryValue != nil {
        return *categoryValue
    }
    if parentValue != nil {
        return *parentValue
    }
    return false // system default
}
```

### Eligibility query
Called once per invite attempt — joined from the booking's service:
```sql
SELECT
    s.allows_participants  AS service_override,
    c.allows_participants  AS category_value,
    cp.allows_participants AS parent_value
FROM bookings b
JOIN services s    ON s.id = b.service_id
JOIN categories c  ON c.id = s.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE b.id = $1;
```

### Service response — expose effective value
`GET /services/:id` and the service card in search results should include `allows_participants: bool` (the resolved effective value, not the raw nullable). The UI uses this to show or hide the invite button without a separate API call.

### Participants module change
In `internal/participants/service.go`, the `Invite` function gains one additional check before the existing validations:
```go
eligible, err := s.repo.ResolveParticipantEligibility(ctx, bookingID)
if err != nil {
    return fmt.Errorf("participants.Invite: %w", err)
}
if !eligible {
    return ErrParticipantsNotAllowed
}
```
This check runs before all other validations (creator check, self-invite check, etc.) so ineligible bookings fail fast.

### New sentinel error
Add to `internal/participants/errors.go`:
```go
var ErrParticipantsNotAllowed = errors.New("participants not allowed for this service category")
```
