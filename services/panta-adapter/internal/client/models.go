package client

type PantaDecimal string

type PantaMarketPriceData struct {
	YesPrice          *PantaDecimal `json:"yesPrice"`
	NoPrice           *PantaDecimal `json:"noPrice"`
	PrimaryYesPrice   *PantaDecimal `json:"primaryYesPrice"`
	PrimaryNoPrice    *PantaDecimal `json:"primaryNoPrice"`
	SecondaryYesPrice *PantaDecimal `json:"secondaryYesPrice"`
	SecondaryNoPrice  *PantaDecimal `json:"secondaryNoPrice"`
}

type PantaPagination struct {
	NextCursor *string `json:"nextCursor"`
}

// PantaMarket is Panta's documented USDC market catalog row.
type PantaMarket struct {
	MarketID         string       `json:"marketId"`
	Category         string       `json:"category"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	Images           []string     `json:"images"`
	Phase            string       `json:"phase"`
	MarketType       string       `json:"marketType"`
	StartTime        int64        `json:"startTime"`
	EndTime          int64        `json:"endTime"`
	ResolutionTime   int64        `json:"resolutionTime"`
	Region           string       `json:"region"`
	Resolved         bool         `json:"resolved"`
	Status           string       `json:"status"`
	VolumeUSDC       PantaDecimal `json:"volumeUsdc"`
	CampaignID       *string      `json:"campaignId"`
	CreatedByPartner bool         `json:"createdByPartner"`
	PantaMarketPriceData
}

type PantaMarketPage struct {
	Items []PantaMarket `json:"items"`
	PantaPagination
}

type PantaMarketQuery struct {
	Category  string
	Status    string
	CreatedBy string
	Cursor    string
	Limit     int
}

type Account struct {
	UserID           string `json:"userId"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	CanCreateMarkets bool   `json:"canCreateMarkets"`
	CreatedAt        string `json:"createdAt"`
	APIKeyID         string `json:"apiKeyId"`
}

type PantaErrorResponse struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Field   string              `json:"field"`
	Fields  map[string][]string `json:"fields"`
}
