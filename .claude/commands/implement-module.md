# Implement a module

Implement the `$ARGUMENTS` module completely and correctly, following the project conventions in CLAUDE.md.

## What to build

For the module named in $ARGUMENTS, create or complete all four files:

- `internal/$ARGUMENTS/model.go` — all structs: DB row models, request DTOs, response DTOs
- `internal/$ARGUMENTS/repository.go` — all SQL queries, using pgx named arguments, no business logic
- `internal/$ARGUMENTS/service.go` — all business logic, calls repository, returns domain errors
- `internal/$ARGUMENTS/handler.go` — Gin handlers, binds requests, calls service, formats JSON responses
- `internal/$ARGUMENTS/errors.go` — sentinel errors specific to this module

## Rules to follow

- All PKs are UUID (`uuid_generate_v4()`)
- All tables have `deleted_at` — all queries filter `WHERE deleted_at IS NULL`
- Monetary amounts in centavos (int), never float
- Wrap errors: `fmt.Errorf("modulename.FunctionName: %w", err)`
- Use sentinel errors from `errors.go` for domain failures
- HTTP error shape: `{"error": "snake_case_code", "message": "human readable"}`
- After implementing, run `go build ./...` to verify it compiles
- Then run `go test ./internal/$ARGUMENTS/...` — write tests if none exist

## Read the relevant ADR first

Before writing any code, load the ADR for this module:
- payments module → `@docs/decisions/ADR-001-payment-method-split.md`
- bookings module → `@docs/decisions/ADR-002-reschedule-keeps-original-confirmed.md`
- reviews module → `@docs/decisions/ADR-003-rating-summaries-trigger.md`
- search module → `@docs/decisions/ADR-004-geohash-feed-cache.md`
- availability module → `@docs/decisions/ADR-005-availability-resolution-priority.md`
- notifications module → `@docs/decisions/ADR-006-async-notifications.md`

Also load `@docs/domain-glossary.md` for correct naming of all variables and functions.

## Key domain rules (check CLAUDE.md and ADRs for full details)

- Booking state machine has exactly 7 states — never add or remove states without checking the full transition diagram
- Slot availability must always be re-checked inside the approval DB transaction
- PayMongo webhook handler must verify signature and deduplicate by event ID
- Never send push notifications synchronously — always enqueue to the notification queue
- `rating_summaries` is updated by trigger — never write to it directly from application code