-- 013_sso_auth.down.sql

DROP TABLE IF EXISTS user_identities;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
