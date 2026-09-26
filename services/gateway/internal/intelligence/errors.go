package intelligence

import "errors"

var (
	ErrInvalidMarket      = errors.New("market data is invalid")
	ErrInvalidDecimal     = errors.New("market decimal is invalid")
	ErrInvalidProbability = errors.New("probability is outside 0 through 1")
)
