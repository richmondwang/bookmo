CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email         TEXT UNIQUE NOT NULL,
  phone         TEXT,
  password_hash TEXT,
  role          TEXT NOT NULL CHECK (role IN ('customer','owner','admin')),
  deleted_at    TIMESTAMP,
  created_at    TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
