package intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type Engine interface {
	Generate(context.Context, []EngineObservation, SignalConfig) ([]Signal, error)
}

type EngineObservation struct {
	MarketID          string
	ObservationID     int64
	ObservedAt        string
	ObservationWindow string
	IsNewMarket       bool
	Previous          *Observation
	WindowPrevious    *Observation
	Current           Observation
}

type rustRequest struct {
	Config       rustSignalConfig  `json:"config"`
	Observations []rustObservation `json:"observations"`
}

type rustSignalConfig struct {
	ProbabilityShiftMinor       string `json:"probability_shift_minor"`
	ProbabilityShiftSignificant string `json:"probability_shift_significant"`
	ProbabilityShiftMajor       string `json:"probability_shift_major"`
	ActivityChangeSignificant   string `json:"activity_change_significant"`
	LiquidityChangeSignificant  string `json:"liquidity_change_significant"`
	ObservationWindowSeconds    uint64 `json:"observation_window_seconds"`
}

type rustObservationValues struct {
	YesProbability *string `json:"yes_probability"`
	NoProbability  *string `json:"no_probability"`
	VolumeUSDC     *string `json:"volume_usdc"`
	Liquidity      *string `json:"liquidity"`
}

type rustObservation struct {
	MarketID          string                 `json:"market_id"`
	ObservationID     int64                  `json:"observation_id"`
	ObservedAt        string                 `json:"observed_at"`
	ObservationWindow string                 `json:"observation_window"`
	IsNewMarket       bool                   `json:"is_new_market"`
	Previous          *rustObservationValues `json:"previous"`
	WindowPrevious    *rustObservationValues `json:"window_previous"`
	Current           rustObservationValues  `json:"current"`
}

type RustEngine struct {
	Path string
}

func (engine RustEngine) Generate(ctx context.Context, observations []EngineObservation, config SignalConfig) ([]Signal, error) {
	if engine.Path == "" {
		return nil, errors.New("MARKET_ENGINE_BIN is required")
	}
	request := rustRequest{
		Config: rustSignalConfig{
			ProbabilityShiftMinor: config.ProbabilityShiftMinor, ProbabilityShiftSignificant: config.ProbabilityShiftSignificant,
			ProbabilityShiftMajor: config.ProbabilityShiftMajor, ActivityChangeSignificant: config.ActivityChangeSignificant,
			LiquidityChangeSignificant: config.LiquidityChangeSignificant,
			ObservationWindowSeconds:   uint64(config.ObservationWindow.Seconds()),
		},
		Observations: make([]rustObservation, 0, len(observations)),
	}
	for _, observation := range observations {
		request.Observations = append(request.Observations, rustObservation{
			MarketID: observation.MarketID, ObservationID: observation.ObservationID,
			ObservedAt: observation.ObservedAt, ObservationWindow: observation.ObservationWindow,
			IsNewMarket: observation.IsNewMarket, Previous: observationValues(observation.Previous),
			WindowPrevious: observationValues(observation.WindowPrevious), Current: *observationValues(&observation.Current),
		})
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode Rust engine input: %w", err)
	}
	command := exec.CommandContext(ctx, engine.Path)
	command.Stdin = bytes.NewReader(input)
	var output limitedBuffer
	command.Stdout = &output
	var stderr limitedBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("Rust engine failed: %w", err)
	}
	var signals []Signal
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	if err := decoder.Decode(&signals); err != nil {
		return nil, fmt.Errorf("decode Rust engine output: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("Rust engine returned trailing output")
	}
	return signals, nil
}

func observationValues(observation *Observation) *rustObservationValues {
	if observation == nil {
		return nil
	}
	return &rustObservationValues{
		YesProbability: observation.YesProbability, NoProbability: observation.NoProbability,
		VolumeUSDC: observation.VolumeUSDC, Liquidity: observation.Liquidity,
	}
}

type limitedBuffer struct {
	bytes.Buffer
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	const maxOutput = 16 << 20
	if buffer.Len()+len(value) > maxOutput {
		return 0, errors.New("Rust engine output exceeds size limit")
	}
	return buffer.Buffer.Write(value)
}
