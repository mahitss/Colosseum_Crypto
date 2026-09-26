package intelligence

import "time"

type MarketSummaryDTO struct {
	ID               string     `json:"id"`
	Source           string     `json:"source"`
	SourceMarketID   string     `json:"source_market_id"`
	Title            string     `json:"title"`
	Description      *string    `json:"description"`
	Category         *string    `json:"category"`
	Status           string     `json:"status"`
	YesProbability   *string    `json:"yes_probability"`
	NoProbability    *string    `json:"no_probability"`
	VolumeUSDC       *string    `json:"volume_usdc"`
	Liquidity        *string    `json:"liquidity"`
	ClosesAt         *time.Time `json:"closes_at"`
	ResolutionStatus *string    `json:"resolution_status"`
}

type ObservationDTO struct {
	ID             int64     `json:"id"`
	MarketID       string    `json:"market_id"`
	Timestamp      time.Time `json:"timestamp"`
	YesProbability *string   `json:"yes_probability"`
	NoProbability  *string   `json:"no_probability"`
	VolumeUSDC     *string   `json:"volume_usdc"`
	Liquidity      *string   `json:"liquidity"`
}

type SignalDTO struct {
	ID                int64     `json:"id"`
	MarketID          string    `json:"market_id"`
	Timestamp         time.Time `json:"timestamp"`
	SignalType        string    `json:"signal_type"`
	Severity          string    `json:"severity"`
	Metric            string    `json:"metric"`
	PreviousValue     *string   `json:"previous_value"`
	CurrentValue      *string   `json:"current_value"`
	AbsoluteChange    *string   `json:"absolute_change"`
	PercentageChange  *string   `json:"percentage_change"`
	PercentagePoints  *string   `json:"percentage_points"`
	ObservationWindow string    `json:"window"`
	Source            string    `json:"source"`
}

type MarketMetricsDTO struct {
	YesProbability   *string    `json:"yes_probability"`
	NoProbability    *string    `json:"no_probability"`
	VolumeUSDC       *string    `json:"volume_usdc"`
	Liquidity        *string    `json:"liquidity"`
	ObservedAt       *time.Time `json:"observed_at"`
	InsufficientData bool       `json:"insufficient_data"`
}

type MarketIntelligenceDTO struct {
	Market            MarketSummaryDTO `json:"market"`
	LatestObservation *ObservationDTO  `json:"latest_observation"`
	RecentSignals     []SignalDTO      `json:"recent_signals"`
	CurrentMetrics    MarketMetricsDTO `json:"current_metrics"`
}

type SignalPageDTO struct {
	Items       []SignalDTO `json:"items"`
	NextCursor  *string     `json:"next_cursor"`
	Total       int64       `json:"total"`
	MarketCount int64       `json:"market_count"`
}

func ToSignalDTO(signal Signal) SignalDTO {
	return SignalDTO{
		ID: signal.ID, MarketID: signal.SourceMarketID, Timestamp: signal.Timestamp,
		SignalType: signal.SignalType, Severity: signal.Severity, Metric: signal.Metric,
		PreviousValue: signal.PreviousValue, CurrentValue: signal.CurrentValue,
		AbsoluteChange: signal.AbsoluteChange, PercentageChange: signal.PercentageChange,
		PercentagePoints: signal.PercentagePoints, ObservationWindow: signal.ObservationWindow,
		Source: signal.Source,
	}
}

func ToSignalPageDTO(page SignalPage) SignalPageDTO {
	items := make([]SignalDTO, 0, len(page.Items))
	for _, signal := range page.Items {
		items = append(items, ToSignalDTO(signal))
	}
	return SignalPageDTO{Items: items, NextCursor: page.NextCursor, Total: page.Total, MarketCount: page.MarketCount}
}

func ToMarketIntelligenceDTO(result MarketIntelligence) MarketIntelligenceDTO {
	market := result.Market
	response := MarketIntelligenceDTO{
		Market: MarketSummaryDTO{
			ID: market.SourceMarketID, Source: market.Source, SourceMarketID: market.SourceMarketID,
			Title: market.Title, Description: market.Description, Category: market.Category,
			Status: market.Status, YesProbability: market.YesProbability, NoProbability: market.NoProbability,
			VolumeUSDC: market.VolumeUSDC, Liquidity: market.Liquidity,
			ClosesAt: market.ClosesAt, ResolutionStatus: market.ResolutionStatus,
		},
		RecentSignals: make([]SignalDTO, 0, len(result.RecentSignals)),
		CurrentMetrics: MarketMetricsDTO{
			YesProbability: result.CurrentMetrics.YesProbability, NoProbability: result.CurrentMetrics.NoProbability,
			VolumeUSDC: result.CurrentMetrics.VolumeUSDC, Liquidity: result.CurrentMetrics.Liquidity,
			ObservedAt: result.CurrentMetrics.ObservedAt, InsufficientData: result.CurrentMetrics.InsufficientData,
		},
	}
	if result.LatestObservation != nil {
		observation := *result.LatestObservation
		response.LatestObservation = &ObservationDTO{
			ID: observation.ID, MarketID: market.SourceMarketID, Timestamp: observation.Timestamp,
			YesProbability: observation.YesProbability, NoProbability: observation.NoProbability,
			VolumeUSDC: observation.VolumeUSDC, Liquidity: observation.Liquidity,
		}
	}
	for _, signal := range result.RecentSignals {
		response.RecentSignals = append(response.RecentSignals, ToSignalDTO(signal))
	}
	return response
}
