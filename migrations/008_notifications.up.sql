CREATE TABLE device_tokens (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token      TEXT NOT NULL,
  platform   TEXT NOT NULL CHECK (platform IN ('ios','android')),
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (user_id, token)
);

CREATE TABLE notifications (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type       TEXT NOT NULL,
  title      TEXT NOT NULL,
  body       TEXT NOT NULL,
  data       JSONB,
  is_read    BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE notification_logs (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  notification_id UUID REFERENCES notifications(id),
  device_token_id UUID REFERENCES device_tokens(id),
  status          TEXT NOT NULL CHECK (status IN ('pending','sent','failed','invalid_token')),
  provider_ref    TEXT,
  error_message   TEXT,
  attempted_at    TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE review_prompts (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id       UUID NOT NULL REFERENCES bookings(id) UNIQUE,
  sent_at          TIMESTAMP,
  reminder_sent_at TIMESTAMP,
  review_id        UUID,
  created_at       TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user   ON notifications(user_id, is_read);
CREATE INDEX idx_device_tokens_user   ON device_tokens(user_id) WHERE is_active = true;
CREATE INDEX idx_notification_logs_status ON notification_logs(status, attempted_at);
