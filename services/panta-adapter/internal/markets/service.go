package markets

import (
	"context"

	"prophet/panta-adapter/internal/client"
	"prophet/types"
)

type pantaReader interface {
	ListMarkets(context.Context, client.PantaMarketQuery) (client.PantaMarketPage, error)
	GetMarket(context.Context, string) (client.PantaMarket, error)
}

type Service struct {
	panta pantaReader
}

func NewService(panta pantaReader) *Service {
	return &Service{panta: panta}
}

func (s *Service) ListMarkets(ctx context.Context, query types.MarketQuery) (types.MarketPage, error) {
	page, err := s.panta.ListMarkets(ctx, client.PantaMarketQuery{
		Category: query.Category, Status: query.Status, CreatedBy: query.CreatedBy,
		Cursor: query.Cursor, Limit: query.Limit,
	})
	if err != nil {
		return types.MarketPage{}, err
	}
	markets := make([]types.Market, 0, len(page.Items))
	for _, market := range page.Items {
		markets = append(markets, MapMarket(market))
	}
	return types.MarketPage{Items: markets, NextCursor: page.NextCursor}, nil
}

func (s *Service) GetMarket(ctx context.Context, id string) (types.Market, error) {
	market, err := s.panta.GetMarket(ctx, id)
	if err != nil {
		return types.Market{}, err
	}
	return MapMarket(market), nil
}

func MapMarket(market client.PantaMarket) types.Market {
	return types.Market{
		ID: market.MarketID, Source: types.SourcePanta, SourceMarketID: market.MarketID,
		Category: market.Category, Title: market.Title,
		Description: market.Description, Images: market.Images, Phase: market.Phase,
		MarketType: market.MarketType, StartTime: market.StartTime, EndTime: market.EndTime,
		ResolutionTime: market.ResolutionTime, Region: market.Region, Resolved: market.Resolved,
		Status: market.Status, VolumeUSDC: toHumanUSDC(&market.VolumeUSDC),
		CampaignID: market.CampaignID, CreatedByPartner: market.CreatedByPartner,
		YesPrice: toHumanUSDC(market.YesPrice), NoPrice: toHumanUSDC(market.NoPrice),
		PrimaryYesPrice: toHumanUSDC(market.PrimaryYesPrice), PrimaryNoPrice: toHumanUSDC(market.PrimaryNoPrice),
		SecondaryYesPrice: toHumanUSDC(market.SecondaryYesPrice), SecondaryNoPrice: toHumanUSDC(market.SecondaryNoPrice),
	}
}

func toHumanUSDC(value *client.PantaDecimal) *types.HumanUSDC {
	if value == nil {
		return nil
	}
	normalized := types.HumanUSDC(*value)
	return &normalized
}
