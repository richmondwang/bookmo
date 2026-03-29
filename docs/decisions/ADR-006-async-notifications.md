# ADR-006: Notifications are always async — never sent in request handlers

## Status
Accepted

## Context

The platform sends push notifications (FCM/APNs) for every significant booking event — approval, rejection, reminders, reschedule outcomes, and more. These notifications can be triggered inside booking and payment request handlers.

Two approaches:

**Option A — Synchronous:** Send the push notification inside the HTTP request handler before returning the response. Simple, no queue needed.

**Option B — Asynchronous:** Enqueue a notification job inside the handler. A separate worker process consumes the queue and handles delivery, retries, and dead-token cleanup.

## Decision

All push notifications are sent asynchronously via a job queue. HTTP request handlers never call FCM or APNs directly.

## Reasoning

FCM and APNs are external services with variable latency and occasional outages. A synchronous push call inside a handler means:
- The booking approval endpoint becomes as slow as FCM's worst-case response time
- A FCM outage causes booking confirmations to fail from the customer's perspective, even though the DB write succeeded
- Retry logic (exponential backoff) cannot be implemented cleanly in a synchronous flow

Mobile networks in the Philippines are variable. Keeping request handlers fast and decoupled from external services is more important here than in markets with reliable connectivity.

## Consequences

- The `notifications` table (in-app feed) is written synchronously inside the handler — it is a local DB write and safe to do inline
- Only the push delivery (FCM/APNs call) is deferred to the worker via the job queue
- The worker handles: delivery, logging to `notification_logs`, deactivating invalid tokens, and retry with exponential backoff (1 min → 5 min → 30 min, max 3 attempts)
- After 3 failed attempts, mark as `failed` in `notification_logs` and stop — do not retry indefinitely
- `invalid_token` errors from FCM/APNs set `device_tokens.is_active = false` immediately — do not retry these
- The scheduler (approval deadline warnings, reminders) enqueues jobs the same way — there is one notification queue, one worker, one delivery path
- In tests, assert that the correct job was enqueued — do not assert that FCM was called
- Queue implementation for MVP: Redis list (`LPUSH` / `BRPOP`). Migrate to RabbitMQ or Kafka in Phase 2 if throughput requires it. The worker interface is the same either way.
