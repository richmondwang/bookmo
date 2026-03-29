# Review module before PR

Review the `$ARGUMENTS` module and check for any issues before it's merged.

## Checks to run

### 1. Build and lint
```bash
go build ./...
go vet ./...
```

### 2. Tests
```bash
go test -race ./internal/$ARGUMENTS/...
```

### 3. Domain rules checklist — verify each one manually

**Soft deletes**
- [ ] Every query that reads data filters `WHERE deleted_at IS NULL`
- [ ] No `DELETE FROM` statements anywhere in the module

**Money handling**
- [ ] All amounts stored and passed as `int` centavos
- [ ] No `float64` used for monetary values
- [ ] Display conversion (÷100) only happens in response DTOs, never in storage

**Error handling**
- [ ] All errors wrapped with context: `fmt.Errorf("module.Function: %w", err)`
- [ ] Sentinel errors used for domain failures, not raw `errors.New` in handlers
- [ ] HTTP responses use `{"error": "snake_case", "message": "..."}` shape

**Security**
- [ ] All owner endpoints guarded by `middleware.RequireRole("owner")`
- [ ] No `customer_id` exposed in anonymous review responses
- [ ] PayMongo webhook verifies signature before processing
- [ ] Webhook events deduplicated via `webhook_events` table

**Booking-specific (if reviewing bookings module)**
- [ ] State machine only allows valid transitions
- [ ] Slot availability re-checked inside approval transaction
- [ ] Partial unique index enforces one pending reschedule per booking
- [ ] `reschedule_attempt_count` checked before creating new request

**Payment-specific (if reviewing payments module)**
- [ ] Card path: authorize → capture (not immediate capture)
- [ ] GCash/Maya path: immediate capture → refund on rejection
- [ ] Refund reason recorded in `refunds` table
- [ ] Reconciliation job reads from `payment_intents`, not bookings

**Notifications (if any notification calls)**
- [ ] No synchronous push in request handlers — must enqueue
- [ ] Dead tokens deactivated on `invalid_token` FCM/APNs response

### 4. Report

After all checks, provide:
- A summary of any issues found (with file and line references)
- A clear PASS or NEEDS FIXES verdict
- Suggested fixes for anything that needs attention