-- 010_profiles_customer_reviews.up.sql
-- Adds: user profile fields, customer_reviews, customer_review_disputes,
--       customer_trust_profiles, and all associated triggers and indexes.

-- ── User profile fields ───────────────────────────────────────────────────────
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS full_name         TEXT,
  ADD COLUMN IF NOT EXISTS bio               TEXT CHECK (char_length(bio) <= 300),
  ADD COLUMN IF NOT EXISTS profile_photo_url TEXT,
  ADD COLUMN IF NOT EXISTS is_verified       BOOLEAN NOT NULL DEFAULT false;

-- ── Customer trust profiles ───────────────────────────────────────────────────
-- Pre-aggregated trust signals per customer. Owned by triggers — never written
-- to directly by application code.
CREATE TABLE customer_trust_profiles (
  customer_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  total_bookings     INT          NOT NULL DEFAULT 0,
  completed_bookings INT          NOT NULL DEFAULT 0,
  cancelled_bookings INT          NOT NULL DEFAULT 0,
  completion_rate    NUMERIC(4,1) NOT NULL DEFAULT 0,
  cancellation_rate  NUMERIC(4,1) NOT NULL DEFAULT 0,
  avg_owner_rating   NUMERIC(2,1) NOT NULL DEFAULT 0,
  total_owner_reviews INT         NOT NULL DEFAULT 0,
  last_updated       TIMESTAMP    NOT NULL DEFAULT now()
);

-- ── Owner reviews of customers ────────────────────────────────────────────────
CREATE TABLE customer_reviews (
  id           UUID     PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id   UUID     NOT NULL REFERENCES bookings(id) UNIQUE,
  customer_id  UUID     NOT NULL REFERENCES users(id),
  owner_id     UUID     NOT NULL REFERENCES owners(id),
  branch_id    UUID     NOT NULL REFERENCES branches(id),
  rating       SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body         TEXT     CHECK (char_length(body) <= 500),
  status       TEXT     NOT NULL DEFAULT 'published'
                          CHECK (status IN ('published','flagged','removed')),
  submitted_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_customer_reviews_customer ON customer_reviews(customer_id, status);
CREATE INDEX idx_customer_reviews_owner    ON customer_reviews(owner_id);
CREATE INDEX idx_customer_reviews_booking  ON customer_reviews(booking_id);

-- ── Customer disputes on owner reviews ───────────────────────────────────────
CREATE TABLE customer_review_disputes (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  customer_review_id  UUID NOT NULL REFERENCES customer_reviews(id),
  customer_id         UUID NOT NULL REFERENCES users(id),
  reason              TEXT NOT NULL CHECK (reason IN (
                        'inaccurate','inappropriate','not_my_booking','retaliation'
                      )),
  details             TEXT CHECK (char_length(details) <= 500),
  status              TEXT NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open','reviewed','resolved')),
  created_at          TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (customer_review_id, customer_id)
);

CREATE INDEX idx_disputes_status ON customer_review_disputes(status, created_at);

-- ── Trigger: update trust profile on booking status change ───────────────────
CREATE FUNCTION update_trust_from_booking() RETURNS TRIGGER AS $$
BEGIN
  IF NEW.status NOT IN ('completed','cancelled') THEN
    RETURN NEW;
  END IF;

  INSERT INTO customer_trust_profiles (customer_id)
  VALUES (NEW.customer_id)
  ON CONFLICT (customer_id) DO NOTHING;

  UPDATE customer_trust_profiles SET
    total_bookings     = (
      SELECT COUNT(*) FROM bookings
      WHERE customer_id = NEW.customer_id
        AND status IN ('confirmed','completed','cancelled')
        AND deleted_at IS NULL
    ),
    completed_bookings = (
      SELECT COUNT(*) FROM bookings
      WHERE customer_id = NEW.customer_id
        AND status = 'completed'
        AND deleted_at IS NULL
    ),
    cancelled_bookings = (
      SELECT COUNT(*) FROM bookings
      WHERE customer_id = NEW.customer_id
        AND status = 'cancelled'
        AND deleted_at IS NULL
    ),
    completion_rate    = ROUND(
      (SELECT COUNT(*) FROM bookings
       WHERE customer_id = NEW.customer_id AND status = 'completed' AND deleted_at IS NULL
      )::numeric /
      NULLIF((
        SELECT COUNT(*) FROM bookings
        WHERE customer_id = NEW.customer_id
          AND status IN ('confirmed','completed','cancelled')
          AND deleted_at IS NULL
      ), 0) * 100, 1),
    cancellation_rate  = ROUND(
      (SELECT COUNT(*) FROM bookings
       WHERE customer_id = NEW.customer_id AND status = 'cancelled' AND deleted_at IS NULL
      )::numeric /
      NULLIF((
        SELECT COUNT(*) FROM bookings
        WHERE customer_id = NEW.customer_id
          AND status IN ('confirmed','completed','cancelled')
          AND deleted_at IS NULL
      ), 0) * 100, 1),
    last_updated       = now()
  WHERE customer_id = NEW.customer_id;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_trust_from_booking
AFTER UPDATE ON bookings
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION update_trust_from_booking();

-- ── Trigger: update trust profile on customer_review change ──────────────────
CREATE FUNCTION update_trust_from_customer_review() RETURNS TRIGGER AS $$
DECLARE
  cid UUID;
BEGIN
  cid := COALESCE(NEW.customer_id, OLD.customer_id);

  INSERT INTO customer_trust_profiles (customer_id)
  VALUES (cid)
  ON CONFLICT (customer_id) DO NOTHING;

  UPDATE customer_trust_profiles SET
    avg_owner_rating    = COALESCE((
      SELECT ROUND(AVG(rating)::numeric, 1)
      FROM customer_reviews
      WHERE customer_id = cid AND status = 'published'
    ), 0),
    total_owner_reviews = (
      SELECT COUNT(*) FROM customer_reviews
      WHERE customer_id = cid AND status = 'published'
    ),
    last_updated        = now()
  WHERE customer_id = cid;

  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_trust_from_customer_review
AFTER INSERT OR UPDATE ON customer_reviews
FOR EACH ROW
EXECUTE FUNCTION update_trust_from_customer_review();
