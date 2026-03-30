# Implementation progress

> This file is maintained by Claude Code. Do not edit manually.
> At the start of any new session, read this file first to determine what has been done and what remains.

## Status: IN PROGRESS

## Module checklist

| Module           | Status  | Notes                                      |
|---|---|---|
| availability     | done    | Merged to main                             |
| bookings         | done    | Merged to main                             |
| payments         | pending | depends on bookings                        |
| notifications    | done    | Merged to main                             |
| search           | pending | depends on services                        |
| reviews          | pending | depends on bookings                        |
| profiles         | done    | Written directly (untracked, on main)      |
| customer_reviews | pending | depends on bookings, profiles              |
| participants     | pending | depends on bookings, notifications         |

## Completed modules

| Module        | Completed    | Notes                                                                                 |
|---|---|---|
| availability  | 2026-03-30   | Implements ADR-005 override→rule→closed chain. Slot capacity math via bookings+locks overlap. |
| notifications | 2026-03-30   | In-app write sync, push enqueued to Redis list `notification_jobs`. Worker is MVP stub. |
| profiles      | 2026-03-30   | Role-aware profile response in service layer. Pre-signed S3 URL is MVP stub. `customer_trust_profiles` is read-only from application code. |
| bookings      | 2026-03-30   | Full 7-state machine. Reschedule follows ADR-002 (original stays confirmed). Slot re-check inside approval transaction. Owner queue JOINs customer_trust_profiles. |

## Last session notes

Implemented in waves:
- Wave 1 (parallel agents): `availability`, `notifications`, `profiles` — all build cleanly
- Wave 2: `bookings` — depends on availability; builds cleanly on main
- Stopped per user request before Wave 3 (payments, reviews, search)

Next session should implement in this order:
1. `payments` (ADR-001: card auth/capture vs GCash/Maya immediate capture)
2. `search` (ADR-004: Geohash-5 feed cache, PostGIS proximity)
3. `reviews` (ADR-003: trigger-owned rating_summaries, never write directly)
4. `customer_reviews` (ADR-007: owner reviews of customers)
5. `participants` (ADR-008 + ADR-009: eligibility check first)

## How to resume

If this file shows any modules as `done`, skip them entirely.
Start from the first module marked `pending` or `in_progress`.
Read the relevant ADR before implementing each module.
