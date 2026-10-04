package markets

import (
	"testing"

	"qevryn/panta-adapter/internal/client"
	"qevryn/types"
)

func TestMapMarketPreservesDocumentedFieldsAndDecimalPrices(t *testing.T) {
	price := client.PantaDecimal("0.52")
	market := MapMarket(client.PantaMarket{
		MarketID: "11111111111111111111111111111111", Title: "Documented market", VolumeUSDC: "20.00",
		PantaMarketPriceData: client.PantaMarketPriceData{YesPrice: &price},
	})
	if market.ID != "11111111111111111111111111111111" || market.Title != "Documented market" {
		t.Fatalf("unexpected mapped market: %#v", market)
	}
	// VolumeUSDC is a pointer because the field is nullable upstream.
	if market.VolumeUSDC == nil || *market.VolumeUSDC != types.HumanUSDC("20.00") {
		t.Fatalf("unexpected mapped volume: %#v", market.VolumeUSDC)
	}
	if market.YesPrice == nil || *market.YesPrice != types.HumanUSDC("0.52") {
		t.Fatalf("unexpected mapped price: %#v", market.YesPrice)
	}
}

