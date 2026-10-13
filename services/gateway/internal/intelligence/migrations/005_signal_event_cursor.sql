-- Migration: 005_signal_event_cursor.sql
-- The worker's bridge pass pairs each persisted signal with a durable
-- signal_events row. It used to find "new" signals with a wall-clock window
-- (now - (syncInterval + tickTimeout)). Any tick that failed after persisting
-- signals but before bridging them left those signals beyond the window
-- forever: observations and signals existed while signal_events, alert_events
-- and notifications stayed empty. The bridge now carries a durable cursor:
-- signal_events.signal_id records which signal row produced the event, so a
-- later tick bridges from the last bridged signal regardless of wall time.
-- The fingerprint UNIQUE constraint remains the dedupe backstop.

ALTER TABLE signal_events ADD COLUMN IF NOT EXISTS signal_id BIGINT REFERENCES signals(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS signal_events_signal_id_unique ON signal_events (signal_id) WHERE signal_id IS NOT NULL;