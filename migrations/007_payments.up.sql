CREATE TABLE payment_intents (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id       UUID NOT NULL REFERENCES bookings(id),
  paymongo_id      TEXT UNIQUE NOT NULL,
  amount_centavos  INT NOT NULL,
  currency         TEXT NOT NULL DEFAULT 'PHP',
  method           TEXT CHECK (method IN ('card','gcash','maya','bank_transfer')),
  method_type      TEXT CHECK (method_type IN ('auth_capture','immediate_capture')),
  status           TEXT NOT NULL CHECK (status IN (
                     'pending','authorized','captured','voided','refunded','failed'
                   )),
  paymongo_status  TEXT,
  captured_at      TIMESTAMP,
  voided_at        TIMESTAMP,
  created_at       TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE refunds (
  id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  payment_intent_id  UUID NOT NULL REFERENCES payment_intents(id),
  booking_id         UUID NOT NULL REFERENCES bookings(id),
  paymongo_refund_id TEXT UNIQUE,
  amount_centavos    INT NOT NULL,
  reason             TEXT CHECK (reason IN (
                       'owner_rejected','owner_cancelled','customer_cancelled',
                       'system_timeout','dispute'
                     )),
  status             TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')),
  created_at         TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE webhook_events (
  id           TEXT PRIMARY KEY,
  type         TEXT,
  processed_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE cancellation_policies (
  id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id         UUID NOT NULL REFERENCES services(id),
  hours_before_start INT NOT NULL,
  refund_percent     INT NOT NULL CHECK (refund_percent BETWEEN 0 AND 100),
  created_at         TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_intents_booking ON payment_intents(booking_id);
CREATE INDEX idx_refunds_booking ON refunds(booking_id);
