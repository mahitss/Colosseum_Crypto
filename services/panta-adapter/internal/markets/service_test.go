package markets

import (
	"testing"

	"prophet/panta-adapter/internal/client"
	"prophet/types"
)

func TestMapMarketPreservesDocumentedFieldsAndDecimalPrices(t *testing.T) {
	price := client.PantaDecimal("0.52")
	market := MapMarket(client.PantaMarket{
		MarketID: "11111111111111111111111111111111", Title: "Documented market", VolumeUSDC: "20.00",
		PantaMarketPriceData: client.PantaMarketPriceData{YesPrice: &price},
	})
	if market.ID != "11111111111111111111111111111111" || market.Title != "Documented market" || market.VolumeUSDC != types.HumanUSDC("20.00") {
		t.Fatalf("unexpected mapped market: %#v", market)
	}
	if market.YesPrice == nil || *market.YesPrice != types.HumanUSDC("0.52") {
		t.Fatalf("unexpected mapped price: %#v", market.YesPrice)
	}
}
