-- 011_booking_participants.up.sql

CREATE TABLE booking_participants (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id    UUID NOT NULL REFERENCES bookings(id),
  user_id       UUID NOT NULL REFERENCES users(id),
  invited_by    UUID NOT NULL REFERENCES users(id),
  status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','accepted','declined','left')),
  invited_at    TIMESTAMP NOT NULL DEFAULT now(),
  responded_at  TIMESTAMP,   -- set on accept or decline
  left_at       TIMESTAMP,   -- set on leave
  UNIQUE (booking_id, user_id)
);

-- Query: all participants on a booking
CREATE INDEX idx_participants_booking ON booking_participants(booking_id, status);

-- Query: all bookings a user has been invited to
CREATE INDEX idx_participants_user ON booking_participants(user_id, status);

-- Query: "booked with" history — accepted, not left, completed bookings
CREATE INDEX idx_participants_accepted ON booking_participants(user_id)
  WHERE status = 'accepted' AND left_at IS NULL;
