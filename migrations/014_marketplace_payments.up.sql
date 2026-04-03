-- 014_marketplace_payments.up.sql

-- ── Platform settings (single row) ───────────────────────────────────────────
CREATE TABLE platform_settings (
  id                     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  default_fee_percent_bp INT  NOT NULL DEFAULT 1000,  -- 1000 bp = 10%
  dispute_window_hours   INT  NOT NULL DEFAULT 48,
  updated_at             TIMESTAMP NOT NULL DEFAULT now()
);

-- Seed the single row
INSERT INTO platform_settings (default_fee_percent_bp, dispute_window_hours)
VALUES (1000, 48);

-- ── Category fee rates ────────────────────────────────────────────────────────
CREATE TABLE category_fee_rates (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  category_id     UUID NOT NULL REFERENCES categories(id) UNIQUE,
  fee_percent_bp  INT  NOT NULL CHECK (fee_percent_bp BETWEEN 0 AND 10000),
  created_at      TIMESTAMP NOT NULL DEFAULT now(),
  updated_at      TIMESTAMP
);

CREATE INDEX idx_category_fee_rates_category ON category_fee_rates(category_id);

-- ── Per-owner negotiated fee overrides ───────────────────────────────────────
CREATE TABLE owner_fee_overrides (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id   UUID NOT NULL REFERENCES owners(id) UNIQUE,
  fee_type   TEXT NOT NULL CHECK (fee_type IN ('percent','flat')),
  fee_value  INT  NOT NULL CHECK (fee_value >= 0),
             -- percent: basis points (500 = 5%), flat: centavos
  notes      TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP
);

-- ── Owner payout accounts ────────────────────────────────────────────────────
CREATE TABLE owner_payout_accounts (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id        UUID NOT NULL REFERENCES owners(id),
  account_type    TEXT NOT NULL CHECK (account_type IN ('gcash','maya','bank')),
  account_name    TEXT NOT NULL,
  account_number  TEXT NOT NULL,
  bank_name       TEXT,
  is_default      BOOLEAN NOT NULL DEFAULT false,
  is_verified     BOOLEAN NOT NULL DEFAULT false,
  verified_at     TIMESTAMP,
  created_at      TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at      TIMESTAMP,
  CONSTRAINT bank_name_required_for_bank
    CHECK (account_type != 'bank' OR bank_name IS NOT NULL)
);

CREATE INDEX idx_payout_accounts_owner   ON owner_payout_accounts(owner_id)
  WHERE deleted_at IS NULL;
CREATE INDEX idx_payout_accounts_default ON owner_payout_accounts(owner_id)
  WHERE is_default = true AND is_verified = true AND deleted_at IS NULL;

-- ── Payout batches ────────────────────────────────────────────────────────────
CREATE TABLE owner_payouts (
  id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id              UUID NOT NULL REFERENCES owners(id),
  payout_account_id     UUID NOT NULL REFERENCES owner_payout_accounts(id),
  total_amount_centavos INT  NOT NULL CHECK (total_amount_centavos > 0),
  earnings_count        INT  NOT NULL CHECK (earnings_count > 0),
  paymongo_payout_id    TEXT UNIQUE,
  status                TEXT NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','processing','paid','failed')),
  scheduled_for         TIMESTAMP NOT NULL,
  initiated_at          TIMESTAMP,
  completed_at          TIMESTAMP,
  failure_reason        TEXT,
  created_at            TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_owner_payouts_owner  ON owner_payouts(owner_id, status);
CREATE INDEX idx_owner_payouts_sched  ON owner_payouts(scheduled_for)
  WHERE status = 'pending';

-- ── Owner earnings per booking ────────────────────────────────────────────────
CREATE TABLE owner_earnings (
  id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id            UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  owner_id              UUID NOT NULL REFERENCES owners(id),
  gross_amount_centavos INT  NOT NULL CHECK (gross_amount_centavos > 0),
  fee_centavos          INT  NOT NULL CHECK (fee_centavos >= 0),
  net_amount_centavos   INT  NOT NULL CHECK (net_amount_centavos >= 0),
  fee_type              TEXT NOT NULL CHECK (fee_type IN ('percent','flat')),
  fee_source            TEXT NOT NULL CHECK (fee_source IN (
                          'owner_override','category_rate','platform_default'
                        )),
  status                TEXT NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','released','disputed','paid_out')),
  released_at           TIMESTAMP,
  payout_id             UUID REFERENCES owner_payouts(id),
  created_at            TIMESTAMP NOT NULL DEFAULT now(),
  CONSTRAINT net_equals_gross_minus_fee
    CHECK (net_amount_centavos = gross_amount_centavos - fee_centavos)
);

CREATE INDEX idx_owner_earnings_owner  ON owner_earnings(owner_id, status);
CREATE INDEX idx_owner_earnings_payout ON owner_earnings(payout_id);
CREATE INDEX idx_owner_earnings_booking ON owner_earnings(booking_id);

-- ── Booking disputes ──────────────────────────────────────────────────────────
CREATE TABLE booking_disputes (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id  UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  customer_id UUID NOT NULL REFERENCES users(id),
  reason      TEXT NOT NULL CHECK (reason IN (
                'service_not_rendered','quality_issue','wrong_service',
                'unauthorized_charge','other'
              )),
  details     TEXT CHECK (char_length(details) <= 1000),
  status      TEXT NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','under_review','resolved_release','resolved_refund')),
  admin_notes TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  resolved_at TIMESTAMP
);

CREATE INDEX idx_booking_disputes_status ON booking_disputes(status, created_at);

-- ── Owners table additions ────────────────────────────────────────────────────
ALTER TABLE owners
  ADD COLUMN IF NOT EXISTS payout_schedule TEXT NOT NULL DEFAULT 'weekly'
    CHECK (payout_schedule IN ('daily','weekly'));

-- ── Bookings table addition ───────────────────────────────────────────────────
ALTER TABLE bookings
  ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;
