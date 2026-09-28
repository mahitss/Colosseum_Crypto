package intelligence

import "time"

const SourcePanta = "panta"

type Market struct {
	ID               string     `json:"id"`
	Source           string     `json:"source"`
	SourceMarketID   string     `json:"source_market_id"`
	Title            string     `json:"title"`
	Description      *string    `json:"description"`
	Category         *string    `json:"category"`
	Status           string     `json:"status"`
	Phase            string     `json:"phase"`
	YesProbability   *string    `json:"yes_probability"`
	NoProbability    *string    `json:"no_probability"`
	Liquidity        *string    `json:"liquidity"`
	VolumeUSDC       *string    `json:"volume_usdc"`
	CreatedAt        *time.Time `json:"created_at"`
	ClosesAt         *time.Time `json:"closes_at"`
	ResolutionStatus *string    `json:"resolution_status"`
}

type Observation struct {
	ID             int64     `json:"id"`
	MarketID       string    `json:"market_id"`
	Timestamp      time.Time `json:"timestamp"`
	YesProbability *string   `json:"yes_probability"`
	NoProbability  *string   `json:"no_probability"`
	VolumeUSDC     *string   `json:"volume_usdc"`
	Liquidity      *string   `json:"liquidity"`
}

type Signal struct {
	ID                int64     `json:"id"`
	MarketID          string    `json:"market_id"`
	SourceMarketID    string    `json:"source_market_id"`
	ObservationID     int64     `json:"observation_id"`
	Timestamp         time.Time `json:"timestamp"`
	SignalType        string    `json:"signal_type"`
	Severity          string    `json:"severity"`
	Metric            string    `json:"metric"`
	PreviousValue     *string   `json:"previous_value"`
	CurrentValue      *string   `json:"current_value"`
	AbsoluteChange    *string   `json:"absolute_change"`
	PercentageChange  *string   `json:"percentage_change"`
	PercentagePoints  *string   `json:"percentage_points"`
	ObservationWindow string    `json:"observation_window"`
	Source            string    `json:"source"`
}

type MarketMetrics struct {
	YesProbability   *string    `json:"yes_probability"`
	NoProbability    *string    `json:"no_probability"`
	VolumeUSDC       *string    `json:"volume_usdc"`
	Liquidity        *string    `json:"liquidity"`
	ObservedAt       *time.Time `json:"observed_at"`
	InsufficientData bool       `json:"insufficient_data"`
}

type MarketIntelligence struct {
	Market            Market        `json:"market"`
	LatestObservation *Observation  `json:"latest_observation"`
	RecentSignals     []Signal      `json:"recent_signals"`
	CurrentMetrics    MarketMetrics `json:"current_metrics"`
}

type SignalQuery struct {
	MarketID        string
	SignalType      string
	Severity        string
	From            *time.Time
	To              *time.Time
	Limit           int
	Cursor          string
	CursorTimestamp *time.Time
}

type SignalPage struct {
	Items       []Signal `json:"items"`
	NextCursor  *string  `json:"next_cursor"`
	Total       int64    `json:"total"`
	MarketCount int64    `json:"market_count"`
}

type SyncStats struct {
	RequestID           string    `json:"request_id"`
	StartedAt           time.Time `json:"started_at"`
	EndedAt             time.Time `json:"ended_at"`
	MarketsFetched      int       `json:"markets_fetched"`
	MarketsNormalized   int       `json:"markets_normalized"`
	ObservationsCreated int       `json:"observations_created"`
	ObservationsSkipped int       `json:"observations_skipped"`
	SignalsGenerated    int       `json:"signals_generated"`
	SignalsPersisted    int       `json:"signals_persisted"`
	Errors              int       `json:"errors"`
}

// Watchlist represents a user-created collection of markets to monitor.
type Watchlist struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WatchlistMarket represents the many-to-many relationship between watchlists and markets.
type WatchlistMarket struct {
	WatchlistID string    `json:"watchlist_id"`
	MarketID    string    `json:"market_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// WatchlistWithMarkets extends Watchlist with market details for detailed views.
type WatchlistWithMarkets struct {
	Watchlist
	Markets []WatchlistMarketDetail `json:"markets"`
}

// WatchlistMarketDetail includes market summary for watchlist views.
type WatchlistMarketDetail struct {
	Market
	LatestSignal *Signal `json:"latest_signal,omitempty"`
}

// SignalEvent represents a durable, fingerprinted signal event in the event stream.
// This is the canonical record of a detected signal, distinct from the transient Signal
// used during sync. SignalEvent includes a fingerprint for deduplication.
type SignalEvent struct {
	ID                int64     `json:"id"`
	MarketID          string    `json:"market_id"`
	SourceMarketID    string    `json:"source_market_id"`
	SignalType        string    `json:"signal_type"`
	Severity          string    `json:"severity"`
	Metric            string    `json:"metric"`
	PreviousValue     *string   `json:"previous_value"`
	CurrentValue      *string   `json:"current_value"`
	AbsoluteChange    *string   `json:"absolute_change"`
	PercentageChange  *string   `json:"percentage_change"`
	PercentagePoints  *string   `json:"percentage_points"`
	ObservationWindow string    `json:"observation_window"`
	ObservationID     int64     `json:"observation_id"`
	ObservedAt        time.Time `json:"observed_at"`
	Fingerprint       string    `json:"fingerprint"`
	Source            string    `json:"source"`
	CreatedAt         time.Time `json:"created_at"`
}

// SignalEventQuery is used to query signal events with filters and pagination.
type SignalEventQuery struct {
	MarketID        string
	WatchlistID     string
	SignalType      string
	Severity        string
	From            *time.Time
	To              *time.Time
	Limit           int
	Cursor          string
	CursorTimestamp *time.Time
}

// SignalEventPage is a paginated response of signal events.
type SignalEventPage struct {
	Items      []SignalEvent `json:"items"`
	NextCursor *string       `json:"next_cursor"`
	Total      int64         `json:"total"`
}

// AlertRule defines a user-configurable alert condition.
// Scope: exactly one of watchlist_id, market_id, or neither (global).
type AlertRule struct {
	ID                         string    `json:"id"`
	UserID                     string    `json:"user_id"`
	WatchlistID                *string   `json:"watchlist_id,omitempty"`
	MarketID                   *string   `json:"market_id,omitempty"`
	Name                       string    `json:"name"`
	Enabled                    bool      `json:"enabled"`
	SignalType                 *string   `json:"signal_type,omitempty"`
	MinimumSeverity            *string   `json:"minimum_severity,omitempty"`
	ProbabilityChangeThreshold *string   `json:"probability_change_threshold,omitempty"`
	ActivityChangeThreshold    *string   `json:"activity_change_threshold,omitempty"`
	LiquidityChangeThreshold   *string   `json:"liquidity_change_threshold,omitempty"`
	CooldownSeconds            int       `json:"cooldown_seconds"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// AlertEvent represents a triggered alert event with deduplication.
type AlertEvent struct {
	ID            int64      `json:"id"`
	AlertRuleID   string     `json:"alert_rule_id"`
	SignalEventID int64      `json:"signal_event_id"`
	MarketID      string     `json:"market_id"`
	Status        string     `json:"status"` // PENDING, DELIVERED, SUPPRESSED, FAILED
	TriggeredAt   time.Time  `json:"triggered_at"`
	DedupeKey     string     `json:"dedupe_key"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Notification represents an in-app notification for a user.
type Notification struct {
	ID           int64      `json:"id"`
	UserID       string     `json:"user_id"`
	AlertEventID *int64     `json:"alert_event_id,omitempty"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	Severity     string     `json:"severity"`
	MarketID     *string    `json:"market_id,omitempty"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RadarQuery is used to query the Signal Radar with filters and pagination.
type RadarQuery struct {
	// UserID is the resolved identity of the caller, not a filter. It is required whenever
	// WatchlistID is set: the radar verifies the watchlist belongs to this user before
	// reading any of its signals and returns ErrWatchlistNotFound otherwise. An empty
	// UserID with a non-empty WatchlistID fails closed rather than skipping the check.
	UserID          string
	MarketID        string
	WatchlistID     string
	SignalType      string
	Severity        string
	From            *time.Time
	To              *time.Time
	Limit           int
	Cursor          string
	CursorTimestamp *time.Time
}

// RadarEvent is a signal event enriched with market summary for radar display.
type RadarEvent struct {
	SignalEvent
	MarketSummary MarketSummary `json:"market_summary"`
	Explanation   string        `json:"explanation"`
	DeepLink      string        `json:"deep_link"`
}

// MarketSummary is a minimal market representation for radar and notifications.
type MarketSummary struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Category       *string `json:"category"`
	YesProbability *string `json:"yes_probability"`
	NoProbability  *string `json:"no_probability"`
	Status         string  `json:"status"`
}

// WatchlistIntelligence aggregates intelligence for a watchlist.
type WatchlistIntelligence struct {
	Watchlist     Watchlist              `json:"watchlist"`
	MarketCount   int                    `json:"market_count"`
	Markets       []WatchlistMarketIntel `json:"markets"`
	RecentSignals []SignalEvent          `json:"recent_signals"`
	SeverityDist  map[string]int         `json:"severity_distribution"`
	LatestUpdate  *time.Time             `json:"latest_update"`
}

// WatchlistMarketIntel is a market with its latest state and signal for watchlist views.
type WatchlistMarketIntel struct {
	Market
	LatestSignal      *SignalEvent `json:"latest_signal,omitempty"`
	LatestObservation *Observation `json:"latest_observation,omitempty"`
}
