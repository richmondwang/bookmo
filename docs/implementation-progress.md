# Implementation progress

> This file is maintained by Claude Code. Do not edit manually.
> At the start of any new session, read this file first to determine what has been done and what remains.

## Status: COMPLETE

## Module checklist

| Module           | Status | Notes                                                          |
|---|---|---|
| availability     | done   | Merged to main                                                 |
| bookings         | done   | Merged to main                                                 |
| payments         | done   | Merged to main                                                 |
| notifications    | done   | Merged to main                                                 |
| search           | done   | Merged to main                                                 |
| reviews          | done   | Merged to main                                                 |
| profiles         | done   | Merged to main                                                 |
| customer_reviews | done   | Merged to main                                                 |
| participants     | done   | Merged to main                                                 |

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

## Notable deviations from ADRs

- **Search cache key**: Used lat/lng rounded to 2 decimal places (~1.1km grid) instead of Geohash-5, as the `mmcloughlin/geohash` library was not in the vendor directory. Functionally equivalent for MVP.
- **Notifications worker**: FCM/APNs delivery is a stub (`sendPush` logs and returns success). No separate `fcm.go`/`apns.go` files — push delivery can be added when real keys are available.
- **Profiles S3**: Pre-signed URL generation is a stub (returns the CDN URL as both upload and CDN URL). Real AWS SDK integration needed when S3 credentials are available.
- **Payments PayMongo**: All PayMongo API calls (Authorize/Capture/Void/Refund) are stubs returning fake IDs. Webhook signature verification is fully implemented. Real HTTP calls needed for production.
- **server.go / worker.go**: Fixed pre-existing invalid `https://` import paths as part of final build fix.

## Last session notes

All 9 modules implemented and merged to main. Full project compiles cleanly (`go build ./...`).
