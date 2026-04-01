# Annotate swagger (retrofit)

> **Note**: This command is for retrofitting annotations onto existing handlers that are missing them — for example, if code was written before ADR-011 was in place, or if a handler was added without annotations by mistake. For new implementation, annotations are written inline as part of `/implement-all`. If you are starting fresh, use `/implement-all` instead.

Add go-swagger annotations to the `$ARGUMENTS` module so the OpenAPI spec can be regenerated with `swagger generate spec`.

## Step 1 — Read conventions

Read `@docs/decisions/ADR-011-go-swagger-annotations.md` before writing a single comment. Follow every convention exactly — wrong annotation syntax produces a broken or empty spec.

## Step 2 — Annotate model.go

Open `internal/$ARGUMENTS/model.go` and add `swagger:model` annotations to every struct that is used as a request body or response body.

For each struct:
- Add a doc comment with a plain-English description of what the struct represents
- Add `swagger:model ModelName` on the line before the struct
- Add field-level comments with `required:`, `example:`, `enum:`, `minimum:`, `maximum:` as appropriate
- Use `omitempty` fields as optional (no `required:` tag)
- UUID fields: `format: uuid`, `example: 550e8400-e29b-41d4-a716-446655440000`
- Timestamp fields: `format: date-time`, `example: 2026-01-13T03:00:00Z`
- Amount fields: add a comment explaining centavos — "Amount in centavos. Divide by 100 for display in PHP."
- Status fields with fixed values: add `enum:` listing all valid values

Also add `swagger:parameters` wrapper structs for each request body and path parameter combination. Name them `{operationID}Body` — one per handler that accepts a body.

Also add `swagger:response` wrapper structs for each distinct response shape. At minimum:
- `errorResponse` (if not already defined in another module)
- `{modelName}Response` — single item
- `{modelName}ListResponse` — array of items

## Step 3 — Annotate handler.go

Open `internal/$ARGUMENTS/handler.go` (and `owner_handler.go` if it exists) and add a `swagger:route` comment above every handler function.

For each handler, include:
- HTTP method and path exactly as registered in the Gin router
- Correct tag from the tag list in ADR-011
- A unique operationID in camelCase (e.g. `listBookings`, `approveBooking`)
- A one-line summary (used as the operation title in the UI)
- A multi-line description covering the business rules — what triggers side effects, what gets validated, what the caller should know
- `Security: bearer:` for all authenticated endpoints (omit for public endpoints like POST /auth/*, GET /search, GET /services/:id/reviews)
- All path and query parameters with types and whether required
- All response codes with their response type names

Reference the `swagger:parameters` structs defined in model.go using the operationID.

## Step 4 — Annotate cmd/api/main.go (first module only)

If this is the first module being annotated, add the global `swagger:meta` block to `cmd/api/main.go` exactly as specified in ADR-011. Include:
- Package doc comment with the full app name: "Kadto — a Booking Platform"
- Schemes, Host, BasePath, Version
- Consumes and Produces
- SecurityDefinitions with bearer apiKey

Skip this step if the `swagger:meta` block already exists in main.go.

## Step 5 — Regenerate and validate

After annotating, run:

```bash
swagger generate spec -o docs/openapi.yaml --scan-models
swagger validate docs/openapi.yaml
```

If validation fails, read the error messages carefully and fix the annotations. Common issues:
- Missing `in:` field on a parameter
- operationID used twice (must be globally unique)
- Response type referenced in `swagger:route` but not defined as `swagger:response`
- Mismatched struct field tags

## Step 6 — Confirm coverage

After validation passes, confirm every handler in the module has a `swagger:route` annotation by running:

```bash
grep -n "func (h \*Handler)" internal/$ARGUMENTS/handler.go | wc -l
grep -n "swagger:route" internal/$ARGUMENTS/handler.go | wc -l
```

Both counts must match. If they don't, find the unannotated handlers and add their annotations.

## Module-specific rules

### auth module
- `POST /auth/register`, `POST /auth/login`, `POST /auth/sso`, `POST /auth/send-otp`, `POST /auth/verify-otp` — no `Security:` (public endpoints)
- `POST /auth/sso` response: document both 200 (success) and 409 (email collision with pending_link_token) explicitly
- `POST /auth/send-otp` — document that it always returns 200 regardless of whether email exists

### payments module
- `POST /payments/webhook` — no `Security:` (PayMongo calls this directly, not the mobile app)
- Document that webhook signature verification happens before any processing
- Amount fields: always note "in centavos"

### bookings module
- Document the full 7-state machine in the Booking model description
- `POST /bookings/lock` — document the 2–5 minute TTL on the lock
- Reschedule approval: document that the new slot is re-checked inside the transaction

### owner module
- All `/owner/*` endpoints require `Security: bearer:` AND the caller must have `role: owner`
- Document the 24-hour approval window on queue endpoints
- Approval queue response: document that customer trust profile is included

### participants module
- `POST /bookings/{id}/participants` — document that 403 is returned when `service.allows_participants` is false
- Document the accept/decline flow and that `left_at` is set on leave

### reviews module
- Document the 14-day submission window on POST /reviews and POST /customer-reviews
- Document that `customer_id` is never returned when `is_anonymous` is true
- Document the 3-flag auto-hide threshold