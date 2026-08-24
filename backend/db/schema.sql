-- =========================================================
-- LAST-MILE DELIVERY TRACKER — NORMALIZED POSTGRES SCHEMA
-- Target: 3NF. App-level RBAC (role column), not DB-level roles.
-- =========================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()

-- ---------------------------------------------------------
-- ENUM TYPES
-- ---------------------------------------------------------
CREATE TYPE user_role        AS ENUM ('customer', 'agent', 'admin');
CREATE TYPE order_type       AS ENUM ('B2B', 'B2C');
CREATE TYPE payment_type     AS ENUM ('PREPAID', 'COD');
CREATE TYPE order_status     AS ENUM (
  'CREATED', 'CONFIRMED', 'ASSIGNED', 'PICKED_UP',
  'IN_TRANSIT', 'OUT_FOR_DELIVERY', 'DELIVERED',
  'FAILED', 'RESCHEDULED', 'CANCELLED'
);
CREATE TYPE agent_availability AS ENUM ('AVAILABLE', 'BUSY', 'OFFLINE');
CREATE TYPE actor_type       AS ENUM ('CUSTOMER', 'AGENT', 'ADMIN', 'SYSTEM');
CREATE TYPE notification_channel AS ENUM ('EMAIL', 'SMS');
CREATE TYPE notification_status  AS ENUM ('PENDING', 'SENT', 'FAILED');

-- ---------------------------------------------------------
-- 1. USERS  (single table, role-discriminated — customer/agent/admin
--    share identity/auth fields; role-specific fields go in
--    agent_profiles to keep this table in 3NF)
-- ---------------------------------------------------------
CREATE TABLE users (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  role              user_role NOT NULL,
  full_name         VARCHAR(120) NOT NULL,
  email             VARCHAR(255) NOT NULL UNIQUE,
  phone             VARCHAR(20)  NOT NULL UNIQUE,
  password_hash     TEXT,              -- null if auth handled by external IdP (e.g. Clerk)
  external_auth_id  VARCHAR(255) UNIQUE, -- Clerk/Auth0 subject id, if used
  email_verified    BOOLEAN NOT NULL DEFAULT FALSE,
  is_active         BOOLEAN NOT NULL DEFAULT TRUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_role ON users(role);

-- ---------------------------------------------------------
-- 2. ADDRESSES (reusable for pickup/drop, normalized out of orders)
-- ---------------------------------------------------------
CREATE TABLE addresses (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES users(id) ON DELETE SET NULL, -- nullable: admin-entered ad hoc addresses
  line1         VARCHAR(255) NOT NULL,
  line2         VARCHAR(255),
  city          VARCHAR(100) NOT NULL,
  state         VARCHAR(100) NOT NULL,
  pincode       VARCHAR(10)  NOT NULL,
  latitude      NUMERIC(9,6),
  longitude     NUMERIC(9,6),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_addresses_pincode ON addresses(pincode);

-- ---------------------------------------------------------
-- 3. ZONES + ZONE_AREAS (admin manages zones, assigns pincodes/areas to zones)
-- ---------------------------------------------------------
CREATE TABLE zones (
  id            SERIAL PRIMARY KEY,
  name          VARCHAR(100) NOT NULL UNIQUE,   -- e.g. "Zone A - Local", "Zone D - Remote"
  description   TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- many pincodes -> one zone (a pincode belongs to exactly one zone)
CREATE TABLE zone_areas (
  id            SERIAL PRIMARY KEY,
  zone_id       INTEGER NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
  pincode       VARCHAR(10) NOT NULL UNIQUE,
  city          VARCHAR(100),
  state         VARCHAR(100)
);
CREATE INDEX idx_zone_areas_zone ON zone_areas(zone_id);

-- ---------------------------------------------------------
-- 4. RATE CARDS (admin-configurable, separate B2B/B2C, versioned)
-- ---------------------------------------------------------
CREATE TABLE rate_cards (
  id              SERIAL PRIMARY KEY,
  order_type      order_type NOT NULL,          -- B2B or B2C
  name            VARCHAR(100) NOT NULL,
  volumetric_divisor INTEGER NOT NULL DEFAULT 5000,
  cod_surcharge_flat NUMERIC(10,2) NOT NULL DEFAULT 0,
  cod_surcharge_pct  NUMERIC(5,2)  NOT NULL DEFAULT 0, -- % of order value
  fuel_surcharge_pct NUMERIC(5,2)  NOT NULL DEFAULT 0, -- % of base charge
  gst_pct            NUMERIC(5,2)  NOT NULL DEFAULT 18.00,
  is_active          BOOLEAN NOT NULL DEFAULT TRUE,
  effective_from     TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to       TIMESTAMPTZ,                -- null = currently active
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- only one ACTIVE rate card per order_type at a time
CREATE UNIQUE INDEX uq_one_active_ratecard_per_type
  ON rate_cards(order_type) WHERE is_active = TRUE;

-- zone-pair pricing rows within a rate card: intra-zone (from=to) and inter-zone rates
CREATE TABLE rate_card_zone_rates (
  id              SERIAL PRIMARY KEY,
  rate_card_id    INTEGER NOT NULL REFERENCES rate_cards(id) ON DELETE CASCADE,
  from_zone_id    INTEGER NOT NULL REFERENCES zones(id),
  to_zone_id      INTEGER NOT NULL REFERENCES zones(id),
  base_price      NUMERIC(10,2) NOT NULL,   -- price for first weight slab (e.g. first 0.5kg)
  base_weight_kg  NUMERIC(6,2)  NOT NULL DEFAULT 0.5,
  additional_price_per_kg NUMERIC(10,2) NOT NULL, -- slab rate beyond base weight (B2C) / per-kg rate (B2B)
  min_chargeable_weight_kg NUMERIC(6,2) NOT NULL DEFAULT 0, -- B2B minimum billed weight
  UNIQUE (rate_card_id, from_zone_id, to_zone_id)
);
CREATE INDEX idx_rczr_lookup ON rate_card_zone_rates(rate_card_id, from_zone_id, to_zone_id);

-- ---------------------------------------------------------
-- 5. AGENT PROFILES (1-1 extension of users where role='agent')
-- ---------------------------------------------------------
CREATE TABLE agent_profiles (
  user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  zone_id           INTEGER REFERENCES zones(id),   -- home/assigned zone
  vehicle_type      VARCHAR(50),
  availability      agent_availability NOT NULL DEFAULT 'OFFLINE',
  current_latitude  NUMERIC(9,6),
  current_longitude NUMERIC(9,6),
  last_location_at  TIMESTAMPTZ,
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_agent_zone_avail ON agent_profiles(zone_id, availability);

-- ---------------------------------------------------------
-- 6. ORDERS (core entity)
-- ---------------------------------------------------------
CREATE TABLE orders (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_number          VARCHAR(20) NOT NULL UNIQUE,  -- human-readable e.g. LMD-20260821-0001

  customer_id           UUID NOT NULL REFERENCES users(id),
  created_by_id         UUID NOT NULL REFERENCES users(id), -- customer or admin (order-on-behalf-of)

  pickup_address_id     UUID NOT NULL REFERENCES addresses(id),
  drop_address_id       UUID NOT NULL REFERENCES addresses(id),
  pickup_zone_id        INTEGER NOT NULL REFERENCES zones(id),
  drop_zone_id          INTEGER NOT NULL REFERENCES zones(id),

  order_type            order_type NOT NULL,
  payment_type          payment_type NOT NULL,

  length_cm             NUMERIC(6,2) NOT NULL,
  breadth_cm            NUMERIC(6,2) NOT NULL,
  height_cm             NUMERIC(6,2) NOT NULL,
  actual_weight_kg      NUMERIC(6,2) NOT NULL,
  volumetric_weight_kg  NUMERIC(6,2) NOT NULL,   -- computed at creation, stored (audit/history)
  chargeable_weight_kg  NUMERIC(6,2) NOT NULL,   -- MAX(actual, volumetric)

  rate_card_id          INTEGER NOT NULL REFERENCES rate_cards(id), -- snapshot reference
  base_charge           NUMERIC(10,2) NOT NULL,
  cod_surcharge         NUMERIC(10,2) NOT NULL DEFAULT 0,
  fuel_surcharge        NUMERIC(10,2) NOT NULL DEFAULT 0,
  gst_amount             NUMERIC(10,2) NOT NULL DEFAULT 0,
  total_charge           NUMERIC(10,2) NOT NULL,
  order_value             NUMERIC(10,2), -- declared goods value (for COD % calc), nullable

  status                order_status NOT NULL DEFAULT 'CREATED',
  assigned_agent_id     UUID REFERENCES users(id),
  assignment_type       VARCHAR(10) CHECK (assignment_type IN ('MANUAL','AUTO')),

  scheduled_delivery_date DATE,           -- set on reschedule after failure
  delivered_at           TIMESTAMPTZ,

  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_orders_agent ON orders(assigned_agent_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_zones ON orders(pickup_zone_id, drop_zone_id);

-- ---------------------------------------------------------
-- 7. ORDER STATUS HISTORY (append-only, immutable audit trail)
--    Never UPDATE/DELETE rows here — enforced via trigger below.
-- ---------------------------------------------------------
CREATE TABLE order_status_history (
  id            BIGSERIAL PRIMARY KEY,
  order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  status        order_status NOT NULL,
  actor_type    actor_type NOT NULL,
  actor_id      UUID REFERENCES users(id),       -- null if actor_type = SYSTEM
  notes         TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_osh_order ON order_status_history(order_id, created_at);

-- immutability guard
CREATE OR REPLACE FUNCTION prevent_history_mutation() RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'order_status_history rows are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_no_update_osh BEFORE UPDATE ON order_status_history
  FOR EACH ROW EXECUTE FUNCTION prevent_history_mutation();
CREATE TRIGGER trg_no_delete_osh BEFORE DELETE ON order_status_history
  FOR EACH ROW EXECUTE FUNCTION prevent_history_mutation();

-- ---------------------------------------------------------
-- 8. FAILED DELIVERY / RESCHEDULE REQUESTS
-- ---------------------------------------------------------
CREATE TABLE reschedule_requests (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  failed_attempt_no INTEGER NOT NULL DEFAULT 1,
  failure_reason    TEXT,
  requested_date    DATE NOT NULL,
  previous_agent_id UUID REFERENCES users(id),
  new_agent_id      UUID REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reschedule_order ON reschedule_requests(order_id);

-- ---------------------------------------------------------
-- 9. NOTIFICATIONS (email/SMS log — one row per attempted send)
-- ---------------------------------------------------------
CREATE TABLE notifications (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  user_id       UUID NOT NULL REFERENCES users(id),
  channel       notification_channel NOT NULL,
  trigger_status order_status NOT NULL,  -- which status change caused this
  status        notification_status NOT NULL DEFAULT 'PENDING',
  sent_at       TIMESTAMPTZ,
  error_message TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_order ON notifications(order_id);

-- ---------------------------------------------------------
-- updated_at auto-touch trigger (users, orders, agent_profiles)
-- ---------------------------------------------------------
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_touch BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_orders_touch BEFORE UPDATE ON orders
  FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_agent_touch BEFORE UPDATE ON agent_profiles
  FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
