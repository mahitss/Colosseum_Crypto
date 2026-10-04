package intelligence

import (
	"testing"

	"qevryn/types"
)

func TestNormalizeMarketUsesOnlyAvailablePantaFields(t *testing.T) {
	volume := types.HumanUSDC("1200.00")
	yes := types.HumanUSDC("0.52")
	no := types.HumanUSDC("0.48")
	market, err := NormalizeMarket(types.Market{
		ID: "11111111111111111111111111111111", SourceMarketID: "11111111111111111111111111111111",
		Title: "Panta market", Status: "open", Phase: "primary",
		VolumeUSDC: &volume, YesPrice: &yes, NoPrice: &no, EndTime: 1798761599,
	})
	if err != nil {
		t.Fatal(err)
	}
	if market.Source != SourcePanta || market.SourceMarketID != "11111111111111111111111111111111" {
		t.Fatalf("unexpected source identity: %#v", market)
	}
	if market.VolumeUSDC == nil || *market.VolumeUSDC != "1200.00" || market.YesProbability == nil || *market.YesProbability != "0.52" {
		t.Fatalf("unexpected Panta values: %#v", market)
	}
	if market.CreatedAt != nil || market.Liquidity != nil || market.ResolutionStatus != nil || market.ClosesAt == nil {
		t.Fatalf("normalizer invented unavailable fields: %#v", market)
	}
}

func TestNormalizeMarketRejectsInvalidInputsWithoutRounding(t *testing.T) {
	for _, market := range []types.Market{
		{ID: "bad-id", Title: "bad", Status: "open"},
		{ID: "11111111111111111111111111111111", Title: "bad", YesPrice: humanUSDC("1.01")},
		{ID: "11111111111111111111111111111111", Title: "bad", VolumeUSDC: humanUSDC("1.0000000000001")},
	} {
		if _, err := NormalizeMarket(market); err == nil {
			t.Errorf("expected invalid market to fail: %#v", market)
		}
	}
}

func humanUSDC(value string) *types.HumanUSDC {
	amount := types.HumanUSDC(value)
	return &amount
}

