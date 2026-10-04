package intelligence

import (
	"math/big"
	"strings"
	"time"

	"qevryn/types"
)

func NormalizeMarket(source types.Market) (Market, error) {
	if source.SourceMarketID == "" || !types.IsBase58Address(source.SourceMarketID) || strings.TrimSpace(source.Title) == "" {
		return Market{}, ErrInvalidMarket
	}
	market := Market{
		Source: SourcePanta, SourceMarketID: source.SourceMarketID,
		Title: source.Title, Status: source.Status, Phase: source.Phase,
	}
	if source.Description != "" {
		market.Description = stringPointer(source.Description)
	}
	if source.Category != "" {
		market.Category = stringPointer(source.Category)
	}
	if source.VolumeUSDC != nil {
		value, err := validateDecimal(string(*source.VolumeUSDC), false)
		if err != nil {
			return Market{}, err
		}
		market.VolumeUSDC = &value
	}
	var err error
	market.YesProbability, err = probability(source.YesPrice)
	if err != nil {
		return Market{}, err
	}
	market.NoProbability, err = probability(source.NoPrice)
	if err != nil {
		return Market{}, err
	}
	if source.EndTime > 0 {
		closes := time.Unix(source.EndTime, 0).UTC()
		market.ClosesAt = &closes
	}
	if source.Resolved {
		market.ResolutionStatus = stringPointer("resolved")
	}
	return market, nil
}

func probability(value *types.HumanUSDC) (*string, error) {
	if value == nil {
		return nil, nil
	}
	validated, err := validateDecimal(string(*value), false)
	if err != nil {
		return nil, err
	}
	rational, ok := new(big.Rat).SetString(validated)
	if !ok || rational.Sign() < 0 || rational.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, ErrInvalidProbability
	}
	return &validated, nil
}

func validateDecimal(value string, allowNegative bool) (string, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "+") {
		return "", ErrInvalidDecimal
	}
	unsigned := value
	if strings.HasPrefix(unsigned, "-") {
		if !allowNegative {
			return "", ErrInvalidDecimal
		}
		unsigned = strings.TrimPrefix(unsigned, "-")
	}
	parts := strings.Split(unsigned, ".")
	if len(parts) > 2 || parts[0] == "" || !allDigits(parts[0]) {
		return "", ErrInvalidDecimal
	}
	if len(parts) == 2 {
		if parts[1] == "" || !allDigits(parts[1]) || len(strings.TrimRight(parts[1], "0")) > 12 {
			return "", ErrInvalidDecimal
		}
	}
	return value, nil
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return len(value) > 0
}

func compareDecimal(left, right *string) (bool, error) {
	if left == nil || right == nil {
		return left == nil && right == nil, nil
	}
	leftNumber, ok := new(big.Rat).SetString(*left)
	if !ok {
		return false, ErrInvalidDecimal
	}
	rightNumber, ok := new(big.Rat).SetString(*right)
	if !ok {
		return false, ErrInvalidDecimal
	}
	return leftNumber.Cmp(rightNumber) == 0, nil
}

func stringPointer(value string) *string { return &value }

