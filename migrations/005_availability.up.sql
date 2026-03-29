CREATE TABLE availability_rules (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id   UUID NOT NULL REFERENCES branches(id),
  day_of_week INT  NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
  start_time  TIME NOT NULL,
  end_time    TIME NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE date_overrides (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id   UUID NOT NULL REFERENCES branches(id),
  date        DATE NOT NULL,
  is_closed   BOOLEAN NOT NULL DEFAULT false,
  open_time   TIME,
  close_time  TIME,
  note        TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE (branch_id, date)
);

CREATE INDEX idx_availability_rules_branch ON availability_rules(branch_id);
CREATE INDEX idx_date_overrides_branch_date ON date_overrides(branch_id, date);
