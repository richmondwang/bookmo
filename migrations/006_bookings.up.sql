CREATE TABLE bookings (
  id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id                  UUID NOT NULL REFERENCES services(id),
  branch_id                   UUID NOT NULL REFERENCES branches(id),
  customer_id                 UUID NOT NULL REFERENCES users(id),
  start_time                  TIMESTAMP NOT NULL,
  end_time                    TIMESTAMP NOT NULL,
  quantity                    INT NOT NULL DEFAULT 1,
  status                      TEXT NOT NULL CHECK (status IN (
                                'pending','awaiting_approval','confirmed',
                                'rejected','cancelled','rescheduled','completed'
                              )),
  payment_method              TEXT CHECK (payment_method IN ('card','gcash','maya','bank_transfer')),
  owner_response_deadline     TIMESTAMP,
  rescheduled_from_booking_id UUID REFERENCES bookings(id),
  reschedule_attempt_count    INT NOT NULL DEFAULT 0,
  rejected_reason             TEXT,
  cancelled_by                TEXT CHECK (cancelled_by IN ('customer','owner','system')),
  currency                    TEXT NOT NULL DEFAULT 'PHP',
  deleted_at                  TIMESTAMP,
  created_at                  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE booking_locks (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  service_id UUID NOT NULL REFERENCES services(id),
  branch_id  UUID NOT NULL REFERENCES branches(id),
  start_time TIMESTAMP NOT NULL,
  end_time   TIMESTAMP NOT NULL,
  quantity   INT NOT NULL DEFAULT 1,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE reschedule_requests (
  id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id     UUID NOT NULL REFERENCES bookings(id),
  requested_by   UUID NOT NULL REFERENCES users(id),
  new_start_time TIMESTAMP NOT NULL,
  new_end_time   TIMESTAMP NOT NULL,
  status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','approved','rejected')),
  created_at     TIMESTAMP NOT NULL DEFAULT now()
);

-- Enforce one pending reschedule per booking
CREATE UNIQUE INDEX idx_one_pending_reschedule
  ON reschedule_requests (booking_id)
  WHERE status = 'pending';

-- GiST index for overlap queries
CREATE INDEX idx_bookings_time
  ON bookings USING GIST (tstzrange(start_time, end_time));

CREATE INDEX idx_bookings_service_status
  ON bookings(service_id, status) WHERE deleted_at IS NULL;

CREATE INDEX idx_bookings_customer
  ON bookings(customer_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_booking_locks_expiry
  ON booking_locks(expires_at);
