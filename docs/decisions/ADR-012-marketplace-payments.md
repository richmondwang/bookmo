# ADR-012: Marketplace payments — owner payout accounts, platform fees, escrow, and payouts

## Status
Accepted

## Context

Kadto operates as a marketplace — customers pay Kadto, Kadto holds the funds, deducts a platform fee, and pays out the remainder to the owner after a dispute window expires. Owners need to register their GCash, Maya, or bank account to receive payouts.

## Decisions

### Money flow
1. Customer pays full booking amount to Kadto via PayMongo (existing flow — ADR-001)
2. Booking completes → dispute window opens (24–48h, configurable per platform)
3. Customer may raise a dispute before window expires
4. If no dispute → funds are released: platform fee deducted, owner earning added to pending balance
5. Owner's payout schedule (daily or weekly) batches all pending earnings into a single PayMongo payout transfer to their registered account

### Dispute window
A `booking_disputes` table is separate from `customer_review_disputes` — this is about money, not review fairness. A dispute holds the payout. Admin resolves: either release (payout proceeds) or refund (customer gets their money back, owner gets nothing for that booking).

The dispute window duration is stored in `platform_settings` as `dispute_window_hours` (default 48). The scheduler checks for bookings where `completed_at + dispute_window_hours < now()` and no open dispute exists, then marks earnings as `released`.

### Platform fee structure — three-level resolution
Resolved in priority order (highest first):

1. `owner_fee_overrides` — a negotiated rate for a specific owner, set by admin. Can be either a flat centavo amount or a percentage.
2. `category_fee_rates` — a default percentage per service category, set by admin.
3. `platform_settings.default_fee_percent` — the global fallback percentage.

Fee type is always resolved to a final centavo amount before storing in `owner_earnings`. Never store a percentage in the earnings record — always compute and store the actual centavo amounts at the time of release.

### Owner payout accounts
An owner can register multiple payout accounts. One is marked `is_default = true` and receives all automated payouts. Account types:

- `gcash` — mobile number, verified via OTP
- `maya` — mobile number, verified via OTP
- `bank` — bank name, account number, account name — verified manually by admin (bank micro-deposit verification is a Phase 2 enhancement)

An account must be `verified` before it can receive payouts. The scheduler skips owners with no verified default account and flags them in `payout_alerts`.

### Payout schedule
Owner chooses `daily` (runs 2am Manila time every day) or `weekly` (runs 2am Manila time every Monday). Stored as `owners.payout_schedule`. Default: `weekly`.

The scheduler batches all `released` earnings for a given owner, sums them, deducts any PayMongo transfer fees, and initiates a single PayMongo payout transfer. One `owner_payouts` row is created per batch.

### PayMongo payout API
PayMongo's Payouts API supports GCash, Maya, and bank transfers in the Philippines. The platform uses this to send funds from Kadto's PayMongo account to the owner's registered account. The `paymongo_payout_id` is stored on `owner_payouts` for reconciliation.

## Schema

```sql
-- Owner payout accounts
owner_payout_accounts (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id        UUID NOT NULL REFERENCES owners(id),
  account_type    TEXT NOT NULL CHECK (account_type IN ('gcash','maya','bank')),
  account_name    TEXT NOT NULL,        -- display name / account holder name
  account_number  TEXT NOT NULL,        -- mobile number or bank account number
  bank_name       TEXT,                 -- null for gcash/maya
  is_default      BOOLEAN NOT NULL DEFAULT false,
  is_verified     BOOLEAN NOT NULL DEFAULT false,
  verified_at     TIMESTAMP,
  created_at      TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at      TIMESTAMP
)

-- Per-owner negotiated fee override (set by admin)
owner_fee_overrides (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id     UUID NOT NULL REFERENCES owners(id) UNIQUE,
  fee_type     TEXT NOT NULL CHECK (fee_type IN ('percent','flat')),
  fee_value    INT  NOT NULL,  -- percent: basis points (500 = 5%), flat: centavos
  notes        TEXT,           -- admin notes on the negotiated rate
  created_at   TIMESTAMP NOT NULL DEFAULT now(),
  updated_at   TIMESTAMP
)

-- Per-category default fee rates (set by admin)
category_fee_rates (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  category_id     UUID NOT NULL REFERENCES categories(id) UNIQUE,
  fee_percent_bp  INT  NOT NULL,  -- basis points: 1000 = 10%
  created_at      TIMESTAMP NOT NULL DEFAULT now(),
  updated_at      TIMESTAMP
)

-- Platform-wide settings (single row)
platform_settings (
  id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  default_fee_percent_bp INT NOT NULL DEFAULT 1000,  -- 1000 bp = 10%
  dispute_window_hours   INT NOT NULL DEFAULT 48,
  updated_at             TIMESTAMP NOT NULL DEFAULT now()
)

-- One earning record per completed booking after dispute window
owner_earnings (
  id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id           UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  owner_id             UUID NOT NULL REFERENCES owners(id),
  gross_amount_centavos INT NOT NULL,   -- full booking amount
  fee_centavos          INT NOT NULL,   -- platform fee deducted
  net_amount_centavos   INT NOT NULL,   -- gross - fee = what owner gets
  fee_type             TEXT NOT NULL CHECK (fee_type IN ('percent','flat')),
  fee_source           TEXT NOT NULL CHECK (fee_source IN ('owner_override','category_rate','platform_default')),
  status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','released','disputed','paid_out')),
  released_at          TIMESTAMP,      -- set when dispute window expires cleanly
  payout_id            UUID REFERENCES owner_payouts(id),
  created_at           TIMESTAMP NOT NULL DEFAULT now()
)

-- One dispute per completed booking
booking_disputes (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id        UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  customer_id       UUID NOT NULL REFERENCES users(id),
  reason            TEXT NOT NULL CHECK (reason IN (
                      'service_not_rendered','quality_issue','wrong_service',
                      'unauthorized_charge','other'
                    )),
  details           TEXT CHECK (char_length(details) <= 1000),
  status            TEXT NOT NULL DEFAULT 'open'
                      CHECK (status IN ('open','under_review','resolved_release','resolved_refund')),
  admin_notes       TEXT,
  created_at        TIMESTAMP NOT NULL DEFAULT now(),
  resolved_at       TIMESTAMP
)

-- Payout batches sent to owner accounts
owner_payouts (
  id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id              UUID NOT NULL REFERENCES owners(id),
  payout_account_id     UUID NOT NULL REFERENCES owner_payout_accounts(id),
  total_amount_centavos INT  NOT NULL,
  earnings_count        INT  NOT NULL,
  paymongo_payout_id    TEXT UNIQUE,
  status                TEXT NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','processing','paid','failed')),
  scheduled_for         TIMESTAMP NOT NULL,
  initiated_at          TIMESTAMP,
  completed_at          TIMESTAMP,
  failure_reason        TEXT,
  created_at            TIMESTAMP NOT NULL DEFAULT now()
)
```

## Fee resolution in Go

```go
// ResolveFee returns the platform fee in centavos for a booking.
// Priority: owner override → category rate → platform default.
func ResolveFee(ctx context.Context, ownerID, categoryID uuid.UUID, grossCentavos int) (feeCentavos int, source string, err error) {
    // 1. Owner override
    override, err := repo.GetOwnerFeeOverride(ctx, ownerID)
    if err == nil {
        if override.FeeType == "flat" {
            return override.FeeValue, "owner_override", nil
        }
        // percent: fee_value is basis points
        return grossCentavos * override.FeeValue / 10000, "owner_override", nil
    }

    // 2. Category rate
    catRate, err := repo.GetCategoryFeeRate(ctx, categoryID)
    if err == nil {
        return grossCentavos * catRate.FeePercentBP / 10000, "category_rate", nil
    }

    // 3. Platform default
    settings, _ := repo.GetPlatformSettings(ctx)
    return grossCentavos * settings.DefaultFeePercentBP / 10000, "platform_default", nil
}
```

Basis points (BP) are used for percentages throughout — 1 BP = 0.01%, so 1000 BP = 10%. This avoids floating-point arithmetic entirely. Always store and compute in integer BP and centavos.

## New API endpoints

```
-- Owner payout account management
GET    /owner/payout-accounts
POST   /owner/payout-accounts           { account_type, account_name, account_number, bank_name? }
PUT    /owner/payout-accounts/:id       { is_default }
DELETE /owner/payout-accounts/:id
POST   /owner/payout-accounts/:id/verify-otp  { otp }   (GCash/Maya only)

-- Owner earnings and payout history
GET    /owner/earnings                  ?status=&from=&to=
GET    /owner/payouts                   payout history

-- Owner payout schedule preference
PUT    /owner/payout-schedule           { schedule: "daily"|"weekly" }

-- Customer dispute
POST   /bookings/:id/dispute            { reason, details }

-- Admin (internal)
GET    /admin/disputes
POST   /admin/disputes/:id/resolve      { resolution: "release"|"refund", admin_notes }
GET    /admin/payout-accounts/pending-verification
POST   /admin/payout-accounts/:id/verify
POST   /admin/fee-overrides             { owner_id, fee_type, fee_value, notes }
PUT    /admin/fee-overrides/:owner_id
GET    /admin/category-fee-rates
PUT    /admin/category-fee-rates/:category_id  { fee_percent_bp }
GET    /admin/platform-settings
PUT    /admin/platform-settings         { default_fee_percent_bp?, dispute_window_hours? }
```

## Consequences

- `owners` table gains `payout_schedule TEXT NOT NULL DEFAULT 'weekly' CHECK (payout_schedule IN ('daily','weekly'))`
- `bookings` table gains `completed_at TIMESTAMP` — set when status transitions to `completed`
- The scheduler gains two new jobs: `ReleaseEarnings` (checks expired dispute windows) and `ProcessPayouts` (batches and initiates PayMongo transfers)
- The payments module gains a `payouts.go` file for PayMongo Payouts API integration
- Bank account verification is admin-manual for MVP — no micro-deposit automation yet
- Fee is always computed and stored in centavos at the moment of release, never recalculated retroactively
- `owner_earnings.net_amount_centavos` must always equal `gross_amount_centavos - fee_centavos` — enforce this as a CHECK constraint
- Basis points prevent float arithmetic — always use integer math: `gross * bp / 10000`
