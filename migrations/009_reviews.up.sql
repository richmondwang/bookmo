CREATE TABLE reviews (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id   UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  service_id   UUID NOT NULL REFERENCES services(id),
  branch_id    UUID NOT NULL REFERENCES branches(id),
  customer_id  UUID NOT NULL REFERENCES users(id),
  rating       SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body         TEXT CHECK (char_length(body) <= 1000),
  is_anonymous BOOLEAN NOT NULL DEFAULT false,
  status       TEXT NOT NULL DEFAULT 'published'
                 CHECK (status IN ('published','flagged','removed')),
  submitted_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE review_responses (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  review_id  UUID NOT NULL REFERENCES reviews(id) UNIQUE,
  owner_id   UUID NOT NULL REFERENCES owners(id),
  body       TEXT NOT NULL CHECK (char_length(body) <= 500),
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP
);

CREATE TABLE review_flags (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  review_id   UUID NOT NULL REFERENCES reviews(id),
  reported_by UUID NOT NULL REFERENCES users(id),
  reason      TEXT CHECK (reason IN ('spam','offensive','fake','irrelevant')),
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (review_id, reported_by)
);

CREATE TABLE rating_summaries (
  service_id    UUID PRIMARY KEY REFERENCES services(id),
  total_reviews INT NOT NULL DEFAULT 0,
  total_rating  INT NOT NULL DEFAULT 0,
  avg_rating    NUMERIC(2,1) NOT NULL DEFAULT 0,
  five_star     INT NOT NULL DEFAULT 0,
  four_star     INT NOT NULL DEFAULT 0,
  three_star    INT NOT NULL DEFAULT 0,
  two_star      INT NOT NULL DEFAULT 0,
  one_star      INT NOT NULL DEFAULT 0,
  last_updated  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_reviews_service ON reviews(service_id, status);
CREATE INDEX idx_reviews_customer ON reviews(customer_id);

-- Trigger to keep rating_summaries current
CREATE FUNCTION update_rating_summary() RETURNS TRIGGER AS $$
DECLARE col TEXT;
BEGIN
  col := CASE NEW.rating
    WHEN 5 THEN 'five_star' WHEN 4 THEN 'four_star'
    WHEN 3 THEN 'three_star' WHEN 2 THEN 'two_star'
    ELSE 'one_star' END;

  IF TG_OP = 'INSERT' THEN
    INSERT INTO rating_summaries (service_id, total_reviews, total_rating, avg_rating)
    VALUES (NEW.service_id, 1, NEW.rating, NEW.rating)
    ON CONFLICT (service_id) DO UPDATE SET
      total_reviews = rating_summaries.total_reviews + 1,
      total_rating  = rating_summaries.total_rating + NEW.rating,
      avg_rating    = ROUND((rating_summaries.total_rating + NEW.rating)::numeric
                       / (rating_summaries.total_reviews + 1), 1),
      last_updated  = now();
    EXECUTE format('UPDATE rating_summaries SET %I = %I + 1 WHERE service_id = $1', col, col)
      USING NEW.service_id;

  ELSIF TG_OP = 'UPDATE' AND OLD.status != 'removed' AND NEW.status = 'removed' THEN
    UPDATE rating_summaries SET
      total_reviews = GREATEST(total_reviews - 1, 0),
      total_rating  = GREATEST(total_rating - OLD.rating, 0),
      avg_rating    = CASE WHEN total_reviews - 1 = 0 THEN 0
                      ELSE ROUND((total_rating - OLD.rating)::numeric / (total_reviews - 1), 1)
                      END,
      last_updated  = now()
    WHERE service_id = OLD.service_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_rating_summary
AFTER INSERT OR UPDATE ON reviews
FOR EACH ROW EXECUTE FUNCTION update_rating_summary();
