CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operators (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS operator_sessions (
    token_hash TEXT PRIMARY KEY,
    operator_id TEXT NOT NULL REFERENCES operators(id) ON UPDATE CASCADE ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_operator_sessions_expiry ON operator_sessions(expires_at, revoked_at);

CREATE TABLE IF NOT EXISTS vehicles (
    id TEXT PRIMARY KEY,
    plate_number TEXT NOT NULL UNIQUE,
    vehicle_type TEXT NOT NULL,
    depot_code TEXT NOT NULL,
    status TEXT NOT NULL,
    capacity_kg INTEGER NOT NULL CHECK (capacity_kg > 0),
    odometer_km INTEGER NOT NULL CHECK (odometer_km >= 0),
    inspection_due_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_vehicles_status_depot ON vehicles(status, depot_code);

CREATE TABLE IF NOT EXISTS routes (
    id TEXT PRIMARY KEY,
    route_code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    zone TEXT NOT NULL,
    required_capacity_kg INTEGER NOT NULL CHECK (required_capacity_kg > 0),
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS shifts (
    id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL REFERENCES routes(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    service_date TEXT NOT NULL,
    start_at TEXT NOT NULL,
    end_at TEXT NOT NULL,
    status TEXT NOT NULL,
    assigned_vehicle_id TEXT REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(route_id, service_date)
);
CREATE INDEX IF NOT EXISTS idx_shifts_date_status ON shifts(service_date, status);

CREATE TABLE IF NOT EXISTS trips (
    id TEXT PRIMARY KEY,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    shift_id TEXT NOT NULL UNIQUE REFERENCES shifts(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    driver_id TEXT NOT NULL REFERENCES drivers(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    status TEXT NOT NULL,
    driver_name TEXT NOT NULL,
    started_at TEXT,
    ended_at TEXT,
    start_odo INTEGER,
    end_odo INTEGER,
    load_kg INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(vehicle_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_trips_vehicle_status ON trips(vehicle_id, status);
CREATE INDEX IF NOT EXISTS idx_trips_started_at ON trips(started_at);

CREATE TABLE IF NOT EXISTS maintenance_orders (
    id TEXT PRIMARY KEY,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    opened_at TEXT NOT NULL,
    due_at TEXT NOT NULL,
    closed_at TEXT,
    notes TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_maintenance_vehicle_status ON maintenance_orders(vehicle_id, status);

CREATE TABLE IF NOT EXISTS incidents (
    id TEXT PRIMARY KEY,
    trip_id TEXT NOT NULL REFERENCES trips(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    resolved_at TEXT,
    summary TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_incidents_status_time ON incidents(status, occurred_at);

CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action TEXT NOT NULL,
    result TEXT NOT NULL,
    request_id TEXT NOT NULL,
    metadata_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_entity_time ON audit_events(entity_type, entity_id, created_at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    response_code INTEGER NOT NULL,
    response_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY(scope, key)
);

CREATE TABLE IF NOT EXISTS outbox_jobs (
    id TEXT PRIMARY KEY,
    job_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_outbox_ready ON outbox_jobs(status, next_attempt_at);

CREATE TABLE IF NOT EXISTS inspections (
    id TEXT PRIMARY KEY,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    inspector TEXT NOT NULL,
    status TEXT NOT NULL,
    inspected_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    score INTEGER NOT NULL DEFAULT 0 CHECK (score >= 0 AND score <= 100),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_inspections_vehicle_time ON inspections(vehicle_id, inspected_at);

CREATE TABLE IF NOT EXISTS inspection_items (
    id TEXT PRIMARY KEY,
    inspection_id TEXT NOT NULL REFERENCES inspections(id) ON UPDATE CASCADE ON DELETE CASCADE,
    item_code TEXT NOT NULL,
    result TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    UNIQUE(inspection_id, item_code)
);

CREATE TABLE IF NOT EXISTS fuel_logs (
    id TEXT PRIMARY KEY,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    fuel_type TEXT NOT NULL,
    quantity REAL NOT NULL CHECK (quantity > 0),
    unit TEXT NOT NULL,
    cost_cents INTEGER NOT NULL CHECK (cost_cents >= 0),
    odometer_km INTEGER NOT NULL CHECK (odometer_km >= 0),
    station_code TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fuel_vehicle_time ON fuel_logs(vehicle_id, recorded_at);

CREATE TABLE IF NOT EXISTS drivers (
    id TEXT PRIMARY KEY,
    employee_no TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    license_class TEXT NOT NULL,
    license_expires_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_drivers_status_expiry ON drivers(status, license_expires_at);

CREATE TABLE IF NOT EXISTS driver_certifications (
    id TEXT PRIMARY KEY,
    driver_id TEXT NOT NULL REFERENCES drivers(id) ON UPDATE CASCADE ON DELETE CASCADE,
    certification_code TEXT NOT NULL,
    vehicle_type TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    UNIQUE(driver_id, certification_code)
);
