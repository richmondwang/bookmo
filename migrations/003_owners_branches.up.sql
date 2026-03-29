CREATE TABLE owners (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id             UUID NOT NULL REFERENCES users(id),
  business_name       TEXT NOT NULL,
  verification_status TEXT NOT NULL DEFAULT 'pending'
                        CHECK (verification_status IN ('pending','verified','rejected')),
  onboarding_step     TEXT NOT NULL DEFAULT 'profile'
                        CHECK (onboarding_step IN ('profile','branch','service','availability','complete')),
  deleted_at          TIMESTAMP,
  created_at          TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE branches (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id   UUID NOT NULL REFERENCES owners(id),
  name       TEXT NOT NULL,
  address    TEXT NOT NULL,
  location   GEOGRAPHY(POINT, 4326) NOT NULL,
  deleted_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_branches_owner ON branches(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_branches_location ON branches USING GIST(location);
