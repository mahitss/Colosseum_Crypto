package intelligence

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

type SignalConfig struct {
	ProbabilityShiftMinor       string
	ProbabilityShiftSignificant string
	ProbabilityShiftMajor       string
	ActivityChangeSignificant   string
	LiquidityChangeSignificant  string
	ObservationWindow           time.Duration
}

func DefaultSignalConfig() SignalConfig {
	return SignalConfig{
		ProbabilityShiftMinor: "0.03", ProbabilityShiftSignificant: "0.05", ProbabilityShiftMajor: "0.10",
		ActivityChangeSignificant: "0.50", LiquidityChangeSignificant: "0.25", ObservationWindow: 24 * time.Hour,
	}
}

func LoadSignalConfig() (SignalConfig, error) {
	config := DefaultSignalConfig()
	set := func(key string, destination *string) {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			*destination = value
		}
	}
	set("PROPHET_SIGNAL_PROBABILITY_SHIFT_MINOR", &config.ProbabilityShiftMinor)
	set("PROPHET_SIGNAL_PROBABILITY_SHIFT_SIGNIFICANT", &config.ProbabilityShiftSignificant)
	set("PROPHET_SIGNAL_PROBABILITY_SHIFT_MAJOR", &config.ProbabilityShiftMajor)
	set("PROPHET_SIGNAL_ACTIVITY_CHANGE_SIGNIFICANT", &config.ActivityChangeSignificant)
	set("PROPHET_SIGNAL_LIQUIDITY_CHANGE_SIGNIFICANT", &config.LiquidityChangeSignificant)
	if raw := strings.TrimSpace(os.Getenv("PROPHET_SIGNAL_WINDOW_SECONDS")); raw != "" {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || seconds < 1 || seconds > int64((30*24*time.Hour)/time.Second) {
			return SignalConfig{}, fmt.Errorf("PROPHET_SIGNAL_WINDOW_SECONDS must be from 1 to 2592000")
		}
		config.ObservationWindow = time.Duration(seconds) * time.Second
	}
	return config, validateSignalConfig(config)
}

func validateSignalConfig(config SignalConfig) error {
	minor, ok := new(big.Rat).SetString(config.ProbabilityShiftMinor)
	if !ok || minor.Sign() <= 0 {
		return fmt.Errorf("probability shift minor threshold must be positive")
	}
	significant, ok := new(big.Rat).SetString(config.ProbabilityShiftSignificant)
	if !ok || significant.Cmp(minor) < 0 {
		return fmt.Errorf("probability shift significant threshold must be at least minor")
	}
	major, ok := new(big.Rat).SetString(config.ProbabilityShiftMajor)
	if !ok || major.Cmp(significant) < 0 {
		return fmt.Errorf("probability shift major threshold must be at least significant")
	}
	activity, ok := new(big.Rat).SetString(config.ActivityChangeSignificant)
	if !ok || activity.Sign() <= 0 {
		return fmt.Errorf("activity change threshold must be positive")
	}
	liquidity, ok := new(big.Rat).SetString(config.LiquidityChangeSignificant)
	if !ok || liquidity.Sign() <= 0 {
		return fmt.Errorf("liquidity change threshold must be positive")
	}
	if config.ObservationWindow <= 0 {
		return fmt.Errorf("observation window must be positive")
	}
	return nil
}
