CREATE TABLE IF NOT EXISTS drivers (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id UUID PRIMARY KEY,
    driver_id UUID NOT NULL UNIQUE REFERENCES drivers(id) ON DELETE CASCADE,
    registration_number TEXT NOT NULL UNIQUE,
    supported_connector_types TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS connectors (
    id UUID PRIMARY KEY,
    station_id UUID NOT NULL REFERENCES stations(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('AC', 'DC')),
    power_kw DOUBLE PRECISION NOT NULL CHECK (power_kw > 0),
    status TEXT NOT NULL CHECK (status IN ('AVAILABLE', 'IN_USE', 'OUT_OF_SERVICE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_connectors_station_status_type
    ON connectors(station_id, status, type);

CREATE TABLE IF NOT EXISTS tariffs (
    id UUID PRIMARY KEY,
    connector_type TEXT NOT NULL UNIQUE CHECK (connector_type IN ('AC', 'DC')),
    minimum_charge_paise BIGINT NOT NULL CHECK (minimum_charge_paise >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tariff_tiers (
    id BIGSERIAL PRIMARY KEY,
    tariff_id UUID NOT NULL REFERENCES tariffs(id) ON DELETE CASCADE,
    position INT NOT NULL,
    up_to_kwh DOUBLE PRECISION,
    rate_paise_per_kwh BIGINT NOT NULL CHECK (rate_paise_per_kwh >= 0),
    UNIQUE(tariff_id, position)
);

CREATE TABLE IF NOT EXISTS promo_codes (
    code TEXT PRIMARY KEY,
    discount_percent INT NOT NULL CHECK (discount_percent BETWEEN 1 AND 100),
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL CHECK (valid_until >= valid_from),
    max_uses INT CHECK (max_uses > 0),
    used_count INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS charging_sessions (
    id UUID PRIMARY KEY,
    driver_id UUID NOT NULL REFERENCES drivers(id),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id),
    station_id UUID NOT NULL REFERENCES stations(id),
    connector_id UUID NOT NULL REFERENCES connectors(id),
    physical_connector_type TEXT NOT NULL CHECK (physical_connector_type IN ('AC', 'DC')),
    billed_connector_type TEXT NOT NULL CHECK (billed_connector_type IN ('AC', 'DC')),
    status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'COMPLETED')),
    promo_code TEXT,
    promo_discount_percent INT NOT NULL DEFAULT 0,
    energy_delivered_kwh DOUBLE PRECISION,
    base_cost_paise BIGINT,
    discount_paise BIGINT,
    final_cost_paise BIGINT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS one_active_session_per_connector
    ON charging_sessions(connector_id) WHERE status = 'ACTIVE';
CREATE UNIQUE INDEX IF NOT EXISTS one_active_session_per_driver
    ON charging_sessions(driver_id) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_sessions_driver_started ON charging_sessions(driver_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_station_started ON charging_sessions(station_id, started_at DESC);

INSERT INTO tariffs (id, connector_type, minimum_charge_paise)
VALUES
    ('00000000-0000-0000-0000-0000000000ac', 'AC', 10000),
    ('00000000-0000-0000-0000-0000000000dc', 'DC', 15000)
ON CONFLICT (connector_type) DO NOTHING;

INSERT INTO tariff_tiers (tariff_id, position, up_to_kwh, rate_paise_per_kwh)
VALUES
    ('00000000-0000-0000-0000-0000000000ac', 1, 10, 1200),
    ('00000000-0000-0000-0000-0000000000ac', 2, 25, 900),
    ('00000000-0000-0000-0000-0000000000ac', 3, NULL, 700),
    ('00000000-0000-0000-0000-0000000000dc', 1, 10, 2000),
    ('00000000-0000-0000-0000-0000000000dc', 2, 25, 1400),
    ('00000000-0000-0000-0000-0000000000dc', 3, NULL, 900)
ON CONFLICT (tariff_id, position) DO NOTHING;
