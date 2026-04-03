-- 014_marketplace_payments.down.sql

ALTER TABLE bookings DROP COLUMN IF EXISTS completed_at;
ALTER TABLE owners   DROP COLUMN IF EXISTS payout_schedule;

DROP TABLE IF EXISTS booking_disputes;
DROP TABLE IF EXISTS owner_earnings;
DROP TABLE IF EXISTS owner_payouts;
DROP TABLE IF EXISTS owner_payout_accounts;
DROP TABLE IF EXISTS owner_fee_overrides;
DROP TABLE IF EXISTS category_fee_rates;
DROP TABLE IF EXISTS platform_settings;
