CREATE TABLE categories (
  id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name      TEXT NOT NULL,
  slug      TEXT UNIQUE NOT NULL,
  icon_url  TEXT,
  parent_id UUID REFERENCES categories(id),
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE services (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  branch_id       UUID NOT NULL REFERENCES branches(id),
  category_id     UUID REFERENCES categories(id),
  name            TEXT NOT NULL,
  description     TEXT,
  min_duration    INT NOT NULL,
  max_duration    INT NOT NULL,
  step_minutes    INT NOT NULL DEFAULT 30,
  capacity        INT NOT NULL DEFAULT 1,
  capacity_type   TEXT NOT NULL CHECK (capacity_type IN ('single','multi')),
  price_per_unit  NUMERIC(12,2) NOT NULL,
  tags            TEXT[],
  search_vec      TSVECTOR,
  deleted_at      TIMESTAMP,
  created_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_services_branch   ON services(branch_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_category ON services(category_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_search   ON services USING GIN(search_vec);
CREATE INDEX idx_services_tags     ON services USING GIN(tags);

CREATE FUNCTION update_services_search_vec() RETURNS TRIGGER AS $$
BEGIN
  NEW.search_vec := to_tsvector('english',
    NEW.name || ' ' || coalesce(NEW.description, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_services_search_vec
BEFORE INSERT OR UPDATE ON services
FOR EACH ROW EXECUTE FUNCTION update_services_search_vec();
