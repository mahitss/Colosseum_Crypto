package types

import "time"

const SourcePanta = "panta"

// HumanUSDC is a decimal-denominated USDC amount, for example "20.00".
type HumanUSDC string

// USDCBaseUnits is an integer count of USDC base units, for example "50000000".
type USDCBaseUnits string

// Market contains the documented catalog data Prophet exposes to callers.
type Market struct {
	ID                string     `json:"id"`
	Category          string     `json:"category"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Images            []string   `json:"images"`
	Phase             string     `json:"phase"`
	MarketType        string     `json:"marketType"`
	StartTime         int64      `json:"startTime"`
	EndTime           int64      `json:"endTime"`
	ResolutionTime    int64      `json:"resolutionTime"`
	Region            string     `json:"region"`
	Resolved          bool       `json:"resolved"`
	Status            string     `json:"status"`
	Source            string     `json:"source"`
	SourceMarketID    string     `json:"source_market_id"`
	VolumeUSDC        *HumanUSDC `json:"volumeUsdc"`
	CreatedAt         *time.Time `json:"created_at"`
	ClosesAt          *time.Time `json:"closes_at"`
	ResolutionStatus  *string    `json:"resolution_status"`
	CampaignID        *string    `json:"campaignId"`
	CreatedByPartner  bool       `json:"createdByPartner"`
	YesPrice          *HumanUSDC `json:"yesPrice"`
	NoPrice           *HumanUSDC `json:"noPrice"`
	PrimaryYesPrice   *HumanUSDC `json:"primaryYesPrice"`
	PrimaryNoPrice    *HumanUSDC `json:"primaryNoPrice"`
	SecondaryYesPrice *HumanUSDC `json:"secondaryYesPrice"`
	SecondaryNoPrice  *HumanUSDC `json:"secondaryNoPrice"`
}

type MarketPage struct {
	Items      []Market `json:"items"`
	NextCursor *string  `json:"nextCursor"`
}

type MarketQuery struct {
	Category  string
	Status    string
	CreatedBy string
	Cursor    string
	Limit     int
}
