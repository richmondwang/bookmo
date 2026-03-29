-- 010_profiles_customer_reviews.down.sql

DROP TRIGGER IF EXISTS trg_trust_from_customer_review ON customer_reviews;
DROP FUNCTION IF EXISTS update_trust_from_customer_review;

DROP TRIGGER IF EXISTS trg_trust_from_booking ON bookings;
DROP FUNCTION IF EXISTS update_trust_from_booking;

DROP TABLE IF EXISTS customer_review_disputes;
DROP TABLE IF EXISTS customer_reviews;
DROP TABLE IF EXISTS customer_trust_profiles;

ALTER TABLE users
  DROP COLUMN IF EXISTS full_name,
  DROP COLUMN IF EXISTS bio,
  DROP COLUMN IF EXISTS profile_photo_url,
  DROP COLUMN IF EXISTS is_verified;
