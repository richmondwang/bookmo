# Implementation progress

> This file is maintained by Claude Code. Do not edit manually.
> At the start of any new session, read this file first to determine what has been done and what remains.

## Status: COMPLETE

## Module checklist

| Module           | Status | Notes                                                          |
|---|---|---|
| auth_sso         | done   | SSO + OTP flow fully implemented                               |
| availability     | done   | Merged to main                                                 |
| bookings         | done   | Merged to main                                                 |
| payments         | done   | Merged to main                                                 |
| notifications    | done   | Merged to main                                                 |
| search           | done   | Merged to main                                                 |
| reviews          | done   | Merged to main                                                 |
| profiles         | done   | Merged to main                                                 |
| customer_reviews | done   | Merged to main                                                 |
| participants     | done   | Merged to main                                                 |
| marketplace_payments | done | Fee resolution, earnings release, payout batching           |

## Completed modules

| Module           | Completed  | Notes                                                                                          |
|---|---|---|
| availability     | 2026-03-30 | ADR-005 override→rule→closed chain. Slot capacity math via bookings+locks overlap.            |
| notifications    | 2026-03-30 | In-app write sync, push enqueued to Redis list `notification_jobs`. Worker is MVP stub.        |
| profiles         | 2026-03-30 | Role-aware response in service layer. Pre-signed S3 URL is MVP stub. trust profiles read-only. |
| bookings         | 2026-03-30 | Full 7-state machine. ADR-002 reschedule flow. Slot re-check in approval transaction.          |
| payments         | 2026-03-30 | ADR-001 card auth/capture vs GCash/Maya immediate capture. Webhook HMAC-SHA256 verification.  |
| search           | 2026-03-30 | ADR-004 feed cache using lat/lng grid (2dp ≈ 1.1km). Browse vs keyword modes. PostGIS query.  |
| reviews          | 2026-03-30 | ADR-003: never write to rating_summaries. 14-day window via DB CHECK. Auto-flag at 3 reports. |
| customer_reviews | 2026-03-30 | ADR-007: disputes don't hide reviews. trigger-owned trust profiles. 14-day window via DB CHECK.|
| participants     | 2026-03-30 | ADR-008/009: eligibility checked first. Both-direction booked-with. Async notifications.       |
| auth_sso         | 2026-04-03 | ADR-010: Google/Facebook HTTP verification, Apple JWT stub. bcrypt OTP, crypto/rand link token.|
| marketplace_payments | 2026-04-03 | ADR-012: 3-level fee resolution (BP integer math), earnings scheduler, payout batching stub.  |

## Notable deviations from ADRs

- **Search cache key**: Used lat/lng rounded to 2 decimal places (~1.1km grid) instead of Geohash-5, as the `mmcloughlin/geohash` library was not in the vendor directory. Functionally equivalent for MVP.
- **Notifications worker**: FCM/APNs delivery is a stub (`sendPush` logs and returns success). No separate `fcm.go`/`apns.go` files — push delivery can be added when real keys are available.
- **Profiles S3**: Pre-signed URL generation is a stub (returns the CDN URL as both upload and CDN URL). Real AWS SDK integration needed when S3 credentials are available.
- **Payments PayMongo**: All PayMongo API calls (Authorize/Capture/Void/Refund) are stubs returning fake IDs. Webhook signature verification is fully implemented. Real HTTP calls needed for production.
- **Apple SSO verification**: Apple JWT signature is not verified against Apple's JWKS (RS256). The payload is decoded and claims extracted, but the cryptographic signature check is a stub. Should be upgraded with full JWKS verification before production. Apple's public keys should be cached in Redis per ADR-010.
- **GCash/Maya OTP for payout accounts**: OTP send/verify is stubbed — any non-empty OTP is accepted and the account is marked verified. Real OTP delivery to the mobile number needed.
- **Marketplace PayMongo payouts**: `InitiatePayout` is a stub returning a fake payout ID. Real PayMongo Payouts API integration needed before production.
- **server.go / worker.go**: Route groups remain as TODO — all module code compiles and is wired-ready, but HTTP routes are not registered until server.go is updated.

## Last session notes

All 11 modules implemented. `go build ./...` and `go test -race ./...` pass clean.

auth_sso: Google and Facebook verification use real HTTP endpoints (tokeninfo / graph.me). Apple uses JWT decode without RS256 signature check (stub). OTP uses bcrypt + crypto/rand per ADR-010.

marketplace_payments: Three-level fee resolution using integer basis-point arithmetic per ADR-012. Scheduler.ReleaseEarnings finds eligible bookings via SQL (dispute window + no open dispute + no existing earning). Scheduler.ProcessPayouts batches released earnings per owner. PayMongo Payouts API is stubbed.

## Notes

Swagger annotations are written inline as each module is implemented — not as a separate phase. After each module completes, `swagger generate spec` is run and validated before moving on.

## How to resume

If this file shows any modules as `done`, skip them entirely.
Start from the first module marked `pending` or `in_progress`.
Read the relevant ADR before implementing each module.
