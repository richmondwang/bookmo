# ADR-010: SSO authentication — Google, Facebook, Apple

## Status
Accepted

## Context

The app supports three SSO providers: Google (all platforms), Facebook (all platforms), and Apple (iOS only, required by App Store guidelines for apps that offer social login). Email + password authentication remains available alongside SSO.

## Decisions

### Token verification approach
The mobile app uses each provider's native SDK to authenticate the user and receives an identity token (Google: `id_token`, Facebook: `access_token`, Apple: `identity_token`). The app sends this token to `POST /auth/sso` — the backend verifies it directly with the provider and never redirects through OAuth web flows. This is the correct pattern for mobile apps.

Verification libraries:
- Google: `google-auth-library` (verifies `id_token` against Google's public keys)
- Facebook: HTTP call to `https://graph.facebook.com/me?access_token=...`
- Apple: Verify JWT signature against Apple's public keys from `https://appleid.apple.com/auth/keys`

### Email collision — merge with verification
When a user signs in with a new provider but the email already exists in `users`, the backend does not silently merge. It returns HTTP 409 with `{ code: "email_exists", pending_link_token: "..." }`. The mobile app presents the user with a verification screen. On success, the new identity is linked and the user is logged in.

Two verification options offered to the user:
1. Re-authenticate with the existing provider (if one exists on the account)
2. Receive a 6-digit OTP to the email address

The `pending_link_token` is a short-lived Redis key (15-minute TTL) containing the provider and `provider_id` waiting to be linked. On verification success, the backend reads this token, links the identity, and issues a JWT.

### Apple Sign In — name only on first sign-in
Apple provides `givenName` and `familyName` only on the very first authorization. The mobile app must include `apple_name: { given_name, family_name }` in the SSO request body on first sign-in. The backend sets `users.full_name` from this on account creation. On subsequent sign-ins, `apple_name` is absent — the backend ignores its absence and uses the stored name.

### Password addition for SSO users
SSO users can add a password via `PUT /auth/password` (authenticated endpoint). This sets `users.password_hash` and allows future email + password login. The `user_identities` rows remain — both auth methods work simultaneously.

### email_verified flag
SSO users have `email_verified = true` on account creation — the provider has already verified ownership. Email + password users have `email_verified = false` until they complete OTP verification.

## New API endpoints

```
POST /auth/sso
  body: { provider, token, role?, apple_name? }
  → 200 { token }                        new or returning user
  → 409 { code: "email_exists", pending_link_token }   collision

POST /auth/sso/verify-link
  body: { pending_link_token, otp? }     OTP path
  body: { pending_link_token, provider_token }  re-auth path
  → 200 { token }

POST /auth/send-otp
  body: { email }
  → 200 (always, don't leak email existence)

POST /auth/verify-otp
  body: { email, otp }
  → 200 { token }    (for email verification flow)

PUT  /auth/password   (authenticated)
  body: { password }
  → 200
```

## Consequences

### Schema
```sql
-- user_identities: one row per linked provider per user
CREATE TABLE user_identities (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider    TEXT NOT NULL CHECK (provider IN ('google','facebook','apple')),
  provider_id TEXT NOT NULL,
  email       TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_id)
);

-- Users table additions
ALTER TABLE users
  ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN full_name      TEXT;   -- if not already added by ADR-007

-- password_hash is already nullable — no change needed
```

### Redis keys for pending operations
```
sso:pending_link:{token}   TTL 15min
  value: { user_id, provider, provider_id, provider_email }

auth:otp:{email}           TTL 10min
  value: { otp_hash, attempts: 0 }
```

OTP is a 6-digit code. Hash it with bcrypt before storing — never store plaintext OTP. Max 3 attempts before the key is deleted and a new OTP must be requested.

### Go service layer
```go
// SSO handler routes based on lookup result
func (s *Service) AuthenticateSSO(ctx context.Context, req SSORequest) (string, error) {
    claims, err := s.verifyProviderToken(ctx, req.Provider, req.Token)
    // claims: { ProviderID, Email, Name }

    identity, err := s.repo.GetIdentityByProvider(ctx, req.Provider, claims.ProviderID)
    if err == nil {
        // Returning user — issue JWT
        return s.issueJWT(ctx, identity.UserID)
    }

    user, err := s.repo.GetUserByEmail(ctx, claims.Email)
    if errors.Is(err, ErrNotFound) {
        // New user — create account + identity
        user = s.createUserFromSSO(ctx, claims, req)
        return s.issueJWT(ctx, user.ID)
    }

    // Email collision — store pending link, return 409
    token := s.storePendingLink(ctx, user.ID, req.Provider, claims.ProviderID)
    return "", &ErrEmailCollision{PendingLinkToken: token}
}
```

### Security rules
- Never log or store the raw provider token after verification — discard immediately
- OTP brute-force protection: max 3 attempts, then delete the OTP and require re-request
- `pending_link_token` is a cryptographically random 32-byte token (hex-encoded) — not sequential
- Apple public keys are cached in Redis with their `exp` TTL — don't fetch on every request
- The `POST /auth/send-otp` endpoint always returns 200 regardless of whether the email exists — never leak account existence through this endpoint
