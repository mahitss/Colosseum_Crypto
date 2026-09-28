-- Migration: 004_watchlists_alerts.sql
-- Watchlists, Signal Events, Alert Rules, Alert Events, and Notifications

-- Watchlists
CREATE TABLE IF NOT EXISTS watchlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT watchlists_user_name_unique UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS watchlists_user_id_idx ON watchlists(user_id);

-- Watchlist Markets
CREATE TABLE IF NOT EXISTS watchlist_markets (
    watchlist_id UUID NOT NULL REFERENCES watchlists(id) ON DELETE CASCADE,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (watchlist_id, market_id)
);

CREATE INDEX IF NOT EXISTS watchlist_markets_market_id_idx ON watchlist_markets(market_id);

-- Signal Events (durable event stream with fingerprinting)
CREATE TABLE IF NOT EXISTS signal_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    signal_type TEXT NOT NULL CHECK (signal_type IN ('NEW_MARKET', 'PROBABILITY_SHIFT', 'ACTIVITY_CHANGE', 'LIQUIDITY_CHANGE', 'MARKET_MOVEMENT')),
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'SIGNIFICANT', 'CRITICAL')),
    metric TEXT NOT NULL,
    previous_value NUMERIC(38, 12),
    current_value NUMERIC(38, 12),
    absolute_change NUMERIC(38, 12),
    percentage_change NUMERIC(38, 12),
    percentage_points NUMERIC(38, 12),
    observation_window TEXT NOT NULL,
    observation_id BIGINT NOT NULL REFERENCES market_observations(id) ON DELETE CASCADE,
    observed_at TIMESTAMPTZ NOT NULL,
    fingerprint TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source = 'panta'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT signal_events_fingerprint_unique UNIQUE (fingerprint)
);

CREATE INDEX IF NOT EXISTS signal_events_market_time_idx ON signal_events (market_id, observed_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS signal_events_recent_idx ON signal_events (observed_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS signal_events_severity_idx ON signal_events (severity, observed_at DESC);
CREATE INDEX IF NOT EXISTS signal_events_type_idx ON signal_events (signal_type, observed_at DESC);

-- Alert Rules
CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    watchlist_id UUID REFERENCES watchlists(id) ON DELETE SET NULL,
    market_id UUID REFERENCES markets(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    signal_type TEXT CHECK (signal_type IN ('NEW_MARKET', 'PROBABILITY_SHIFT', 'ACTIVITY_CHANGE', 'LIQUIDITY_CHANGE', 'MARKET_MOVEMENT')),
    minimum_severity TEXT CHECK (minimum_severity IN ('INFO', 'WATCH', 'SIGNIFICANT', 'CRITICAL')),
    probability_change_threshold NUMERIC(38, 12),
    activity_change_threshold NUMERIC(38, 12),
    liquidity_change_threshold NUMERIC(38, 12),
    cooldown_seconds INTEGER NOT NULL DEFAULT 1800,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT alert_rules_valid CHECK (
        (watchlist_id IS NOT NULL AND market_id IS NULL) OR
        (watchlist_id IS NULL AND market_id IS NOT NULL) OR
        (watchlist_id IS NULL AND market_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS alert_rules_user_id_idx ON alert_rules(user_id);
CREATE INDEX IF NOT EXISTS alert_rules_watchlist_id_idx ON alert_rules(watchlist_id);
CREATE INDEX IF NOT EXISTS alert_rules_market_id_idx ON alert_rules(market_id);

-- Alert Events
CREATE TABLE IF NOT EXISTS alert_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    alert_rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    signal_event_id BIGINT NOT NULL REFERENCES signal_events(id) ON DELETE CASCADE,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'DELIVERED', 'SUPPRESSED', 'FAILED')) DEFAULT 'PENDING',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dedupe_key TEXT NOT NULL,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT alert_events_dedupe_unique UNIQUE (alert_rule_id, dedupe_key)
);

CREATE INDEX IF NOT EXISTS alert_events_rule_idx ON alert_events(alert_rule_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS alert_events_market_idx ON alert_events(market_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS alert_events_status_idx ON alert_events(status, triggered_at DESC);

-- In-App Notifications
CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id TEXT NOT NULL,
    alert_event_id BIGINT REFERENCES alert_events(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'SIGNIFICANT', 'CRITICAL')),
    market_id UUID REFERENCES markets(id) ON DELETE SET NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS notifications_unread_idx ON notifications(user_id, read_at) WHERE read_at IS NULL;
