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
