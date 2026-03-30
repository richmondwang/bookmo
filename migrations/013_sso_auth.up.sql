-- 013_sso_auth.up.sql

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;

-- SSO users created from this point forward are email_verified = true.
-- Existing email/password users default to false (they should verify via OTP).
-- Uncomment the line below if you want to treat all existing users as verified:
-- UPDATE users SET email_verified = true;

CREATE TABLE user_identities (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider    TEXT NOT NULL CHECK (provider IN ('google','facebook','apple')),
  provider_id TEXT NOT NULL,
  email       TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_id)
);

CREATE INDEX idx_user_identities_user     ON user_identities(user_id);
CREATE INDEX idx_user_identities_provider ON user_identities(provider, provider_id);
