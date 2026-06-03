-- AI Retail Store Intelligence Platform — PostgreSQL Schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE stores (
    store_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(64) DEFAULT 'UTC',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE cameras (
    camera_id VARCHAR(64) PRIMARY KEY,
    store_id VARCHAR(64) REFERENCES stores(store_id),
    zone_map JSONB DEFAULT '{}',
    stream_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE visitors (
    visitor_id VARCHAR(64) PRIMARY KEY,
    store_id VARCHAR(64) REFERENCES stores(store_id),
    first_seen TIMESTAMPTZ,
    last_seen TIMESTAMPTZ,
    total_sessions INT DEFAULT 0,
    is_staff BOOLEAN DEFAULT FALSE
);

CREATE TABLE visitor_sessions (
    session_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    visitor_id VARCHAR(64) REFERENCES visitors(visitor_id),
    store_id VARCHAR(64) REFERENCES stores(store_id),
    session_seq INT NOT NULL DEFAULT 1,
    entry_at TIMESTAMPTZ,
    exit_at TIMESTAMPTZ,
    dwell_ms BIGINT DEFAULT 0,
    is_reentry BOOLEAN DEFAULT FALSE,
    converted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE store_events (
    event_id UUID PRIMARY KEY,
    store_id VARCHAR(64) NOT NULL,
    camera_id VARCHAR(64),
    visitor_id VARCHAR(64),
    event_type VARCHAR(32) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    zone_id VARCHAR(64),
    dwell_ms BIGINT DEFAULT 0,
    is_staff BOOLEAN DEFAULT FALSE,
    confidence REAL,
    metadata JSONB DEFAULT '{}',
    ingested_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(event_id)
);

CREATE INDEX idx_store_events_store_ts ON store_events(store_id, timestamp DESC);
CREATE INDEX idx_store_events_visitor ON store_events(visitor_id, timestamp DESC);
CREATE INDEX idx_store_events_type ON store_events(event_type);

CREATE TABLE zone_metrics (
    id SERIAL PRIMARY KEY,
    store_id VARCHAR(64) NOT NULL,
    zone_id VARCHAR(64) NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    enter_count INT DEFAULT 0,
    exit_count INT DEFAULT 0,
    avg_dwell_ms BIGINT DEFAULT 0,
    UNIQUE(store_id, zone_id, bucket_start)
);

CREATE TABLE queue_metrics (
    id SERIAL PRIMARY KEY,
    store_id VARCHAR(64) NOT NULL,
    zone_id VARCHAR(64) DEFAULT 'BILLING',
    bucket_start TIMESTAMPTZ NOT NULL,
    max_depth INT DEFAULT 0,
    avg_depth REAL DEFAULT 0,
    UNIQUE(store_id, zone_id, bucket_start)
);

CREATE TABLE funnel_metrics (
    id SERIAL PRIMARY KEY,
    store_id VARCHAR(64) NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    entries INT DEFAULT 0,
    exits INT DEFAULT 0,
    conversions INT DEFAULT 0,
    conversion_rate REAL DEFAULT 0,
    UNIQUE(store_id, bucket_start)
);

CREATE TABLE anomalies (
    anomaly_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id VARCHAR(64) NOT NULL,
    anomaly_type VARCHAR(64) NOT NULL,
    severity VARCHAR(16) DEFAULT 'MEDIUM',
    description TEXT,
    metadata JSONB DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_anomalies_store ON anomalies(store_id, detected_at DESC);

CREATE TABLE heatmap_cells (
    id SERIAL PRIMARY KEY,
    store_id VARCHAR(64) NOT NULL,
    zone_id VARCHAR(64) NOT NULL,
    grid_x INT NOT NULL,
    grid_y INT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    visit_count INT DEFAULT 0,
    UNIQUE(store_id, zone_id, grid_x, grid_y, bucket_start)
);

-- Seed default store
INSERT INTO stores (store_id, name) VALUES ('STORE_BLR_002', 'Bangalore Flagship')
ON CONFLICT DO NOTHING;

INSERT INTO cameras (camera_id, store_id, stream_url) VALUES
('CAM_ENTRY_01', 'STORE_BLR_002', '/data/sample_retail.mp4')
ON CONFLICT DO NOTHING;
