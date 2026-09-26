CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source TEXT NOT NULL CHECK (source = 'panta'),
    source_market_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    category TEXT,
    status TEXT NOT NULL,
    phase TEXT NOT NULL,
    yes_probability NUMERIC(38, 12),
    no_probability NUMERIC(38, 12),
    liquidity NUMERIC(38, 12),
    volume_usdc NUMERIC(38, 12),
    created_at TIMESTAMPTZ,
    closes_at TIMESTAMPTZ,
    resolution_status TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT markets_source_market_unique UNIQUE (source, source_market_id),
    CONSTRAINT markets_yes_probability_range CHECK (yes_probability IS NULL OR yes_probability BETWEEN 0 AND 1),
    CONSTRAINT markets_no_probability_range CHECK (no_probability IS NULL OR no_probability BETWEEN 0 AND 1),
    CONSTRAINT markets_liquidity_nonnegative CHECK (liquidity IS NULL OR liquidity >= 0),
    CONSTRAINT markets_volume_nonnegative CHECK (volume_usdc IS NULL OR volume_usdc >= 0)
);

CREATE INDEX IF NOT EXISTS markets_source_market_id_idx ON markets (source_market_id);
CREATE INDEX IF NOT EXISTS markets_updated_at_idx ON markets (updated_at DESC);

CREATE TABLE IF NOT EXISTS market_observations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    observed_at TIMESTAMPTZ NOT NULL,
    yes_probability NUMERIC(38, 12),
    no_probability NUMERIC(38, 12),
    volume_usdc NUMERIC(38, 12),
    liquidity NUMERIC(38, 12),
    CONSTRAINT market_observations_market_time_unique UNIQUE (market_id, observed_at),
    CONSTRAINT market_observations_yes_probability_range CHECK (yes_probability IS NULL OR yes_probability BETWEEN 0 AND 1),
    CONSTRAINT market_observations_no_probability_range CHECK (no_probability IS NULL OR no_probability BETWEEN 0 AND 1),
    CONSTRAINT market_observations_volume_nonnegative CHECK (volume_usdc IS NULL OR volume_usdc >= 0),
    CONSTRAINT market_observations_liquidity_nonnegative CHECK (liquidity IS NULL OR liquidity >= 0)
);

CREATE INDEX IF NOT EXISTS market_observations_market_time_idx ON market_observations (market_id, observed_at DESC);

CREATE TABLE IF NOT EXISTS signals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    observation_id BIGINT NOT NULL REFERENCES market_observations(id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL,
    signal_type TEXT NOT NULL CHECK (signal_type IN ('NEW_MARKET', 'PROBABILITY_SHIFT', 'ACTIVITY_CHANGE', 'LIQUIDITY_CHANGE', 'MARKET_MOVEMENT')),
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'SIGNIFICANT', 'CRITICAL')),
    metric TEXT NOT NULL,
    previous_value NUMERIC(38, 12),
    current_value NUMERIC(38, 12),
    absolute_change NUMERIC(38, 12),
    percentage_change NUMERIC(38, 12),
    percentage_points NUMERIC(38, 12),
    observation_window TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source = 'panta'),
    CONSTRAINT signals_observation_type_metric_unique UNIQUE (market_id, observation_id, signal_type, metric)
);

CREATE INDEX IF NOT EXISTS signals_market_time_idx ON signals (market_id, timestamp DESC, id DESC);
CREATE INDEX IF NOT EXISTS signals_recent_idx ON signals (timestamp DESC, id DESC);
CREATE INDEX IF NOT EXISTS signals_type_time_idx ON signals (signal_type, timestamp DESC, id DESC);
