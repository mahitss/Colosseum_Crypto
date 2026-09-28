// Package intelligence alert logic.
//
// This file contains ONLY pure, deterministic logic. Every function here is a
// total function of its inputs: the same input values always produce the same
// output values, with no dependence on wall-clock time, randomness, goroutine
// scheduling, map iteration order, locale, or network state.
//
// In particular:
//
//   - No LLM is ever consulted to decide whether an alert rule matched.
//     MatchesRule is a straightforward, auditable predicate over the rule
//     definition and the signal event, so a user can always predict why an
//     alert did or did not fire.
//   - No language model is used to write ExplainSignal. Sentences are assembled
//     mechanically from the structured numeric fields that are actually
//     present on the event. A field that is nil is omitted from the sentence
//     rather than guessed at, so explanations never invent values.
//   - All monetary and probability arithmetic uses math/big.Rat. Money and
//     probability values are exact decimals, and float64 would introduce
//     rounding error that could flip a threshold comparison at the boundary.
//     Formatting is done with big.Rat.FloatString, which is exact rational
//     rounding with no intermediate binary floating point step.
//
// The functions here perform no database access and no HTTP calls. Callers
// (repositories, services, HTTP handlers) own I/O and are responsible for
// fetching the rows these helpers operate on.
package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Signal types recognised by the detection engine. These mirror the
// CHECK constraint on signal_events.signal_type in the schema.
const (
	SignalTypeNewMarket        = "NEW_MARKET"
	SignalTypeProbabilityShift = "PROBABILITY_SHIFT"
	SignalTypeActivityChange   = "ACTIVITY_CHANGE"
	SignalTypeLiquidityChange  = "LIQUIDITY_CHANGE"
	SignalTypeMarketMovement   = "MARKET_MOVEMENT"
)

// Severity levels, ordered from least to most urgent. These mirror the CHECK
// constraint on signal_events.severity in the schema.
const (
	SeverityInfo        = "INFO"
	SeverityWatch       = "WATCH"
	SeveritySignificant = "SIGNIFICANT"
	SeverityCritical    = "CRITICAL"
)

const (
	// maxAlertRuleNameLength bounds the human-supplied rule name.
	maxAlertRuleNameLength = 120
	// maxAlertCooldownSeconds is seven days, the longest cooldown we accept.
	maxAlertCooldownSeconds = 7 * 24 * 60 * 60
	// fingerprintSeparator delimits the components of a fingerprint preimage.
	// The "|" character is not a legal character in any component value, so
	// distinct component tuples cannot collide by concatenation.
	fingerprintSeparator = "|"
)

// validSignalTypes is the closed set of accepted signal types.
var validSignalTypes = map[string]bool{
	SignalTypeNewMarket:        true,
	SignalTypeProbabilityShift: true,
	SignalTypeActivityChange:   true,
	SignalTypeLiquidityChange:  true,
	SignalTypeMarketMovement:   true,
}

// validSeverities is the closed set of accepted severity levels, in ascending
// order of urgency.
var validSeverities = []string{
	SeverityInfo,
	SeverityWatch,
	SeveritySignificant,
	SeverityCritical,
}

// hundred is the exact rational 100, used to convert ratios such as 0.412
// into human-facing percentages such as 41.2.
var hundred = big.NewRat(100, 1)

// SeverityRank maps a severity string to an integer rank so severities can be
// compared with a simple numeric comparison. INFO=1, WATCH=2, SIGNIFICANT=3,
// CRITICAL=4. An unrecognised severity returns 0, which sorts below every
// valid severity: unknown input never satisfies a minimum-severity filter.
func SeverityRank(severity string) int {
	switch severity {
	case SeverityInfo:
		return 1
	case SeverityWatch:
		return 2
	case SeveritySignificant:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

// SignalFingerprint returns a stable, deterministic identity for a signal
// event: the lowercase hex SHA-256 of its market, type, observation, metric and
// current value joined with a delimiter.
//
// The fingerprint is the deduplication key for the event stream, so it must
// depend on exactly the attributes that make two events the same observation.
// It deliberately excludes database-assigned IDs (SignalEvent.ID) and
// timestamps (CreatedAt, ObservedAt), so re-running detection for the same
// underlying observation yields the same fingerprint and is deduplicated
// rather than duplicated.
func SignalFingerprint(event SignalEvent) string {
	components := []string{
		event.MarketID,
		event.SignalType,
		strconv.FormatInt(event.ObservationID, 10),
		event.Metric,
		derefOrEmpty(event.CurrentValue),
	}
	digest := sha256.Sum256([]byte(strings.Join(components, fingerprintSeparator)))
	return hex.EncodeToString(digest[:])
}

// AlertDedupeKey returns the deduplication key used to suppress repeat
// deliveries of the same alert for the same signal event. It is the lowercase
// hex SHA-256 of the rule ID and the event fingerprint joined with a delimiter.
//
// Hashing rather than concatenating keeps the key a fixed 64 characters
// regardless of how long the rule ID is, and removes any possibility of
// delimiter ambiguity between the two components. The schema already pairs
// this key with alert_rule_id in a unique constraint, so repeating the rule ID
// inside the hash costs nothing and makes the key self-describing.
func AlertDedupeKey(ruleID string, event SignalEvent) string {
	preimage := strings.Join([]string{ruleID, SignalFingerprint(event)}, fingerprintSeparator)
	digest := sha256.Sum256([]byte(preimage))
	return hex.EncodeToString(digest[:])
}

// ExplainSignal renders a short human-readable sentence describing what
// happened, using only the structured values actually present on the event.
//
// The function never invents a value. A sentence is assembled from the first
// applicable branch below, and any component whose backing field is nil is
// dropped from the sentence rather than filled in. Percentage rendering
// multiplies by 100 using exact rational arithmetic and formats the result
// with big.Rat.FloatString, so no value ever passes through a float64.
func ExplainSignal(event SignalEvent) string {
	if event.SignalType == SignalTypeNewMarket {
		// A new-market event carries no title and no prior values, so there
		// is nothing more to say without guessing.
		return "New market discovered in the Prophet catalog."
	}

	previous, hasPrevious := parseDecimalRat(event.PreviousValue)
	current, hasCurrent := parseDecimalRat(event.CurrentValue)
	both := hasPrevious && hasCurrent

	switch {
	// A probability shift is reported in percentage points because that is the
	// unit a reader of a prediction market actually cares about.
	case event.PercentagePoints != nil && both:
		points, ok := parseDecimalRat(event.PercentagePoints)
		if !ok {
			break
		}
		return fmt.Sprintf(
			"YES probability moved from %s to %s (%s percentage points).",
			formatPercent(previous),
			formatPercent(current),
			formatSignedPoints(points),
		)

	// A relative change is reported with a directional verb chosen from the
	// sign of the change; the magnitude is always shown unsigned.
	case event.PercentageChange != nil:
		change, ok := parseDecimalRat(event.PercentageChange)
		if !ok {
			break
		}
		direction := "increased"
		if change.Sign() < 0 {
			direction = "decreased"
		}
		if both {
			return fmt.Sprintf(
				"Activity %s %s (from %s to %s).",
				direction,
				formatMagnitudePercent(change),
				derefOrEmpty(event.PreviousValue),
				derefOrEmpty(event.CurrentValue),
			)
		}
		return fmt.Sprintf("Activity %s %s.", direction, formatMagnitudePercent(change))

	// Fall back to the raw absolute delta. The stored value is echoed verbatim
	// so the sentence carries the exact figure of record.
	case event.AbsoluteChange != nil:
		if both {
			return fmt.Sprintf(
				"Liquidity changed by %s (from %s to %s).",
				derefOrEmpty(event.AbsoluteChange),
				derefOrEmpty(event.PreviousValue),
				derefOrEmpty(event.CurrentValue),
			)
		}
		return fmt.Sprintf("Liquidity changed by %s.", derefOrEmpty(event.AbsoluteChange))
	}

	// No usable delta. If the event carries no measurements at all it conveys
	// only the fact that an observation exists.
	if event.PreviousValue == nil && event.CurrentValue == nil &&
		event.AbsoluteChange == nil && event.PercentageChange == nil &&
		event.PercentagePoints == nil {
		return "New observation recorded for this market."
	}

	// Some values are present but none of them form a recognisable delta.
	// Report the current reading alone rather than inferring a movement.
	if hasCurrent {
		return fmt.Sprintf("Current value for this market is %s.", derefOrEmpty(event.CurrentValue))
	}
	return "Signal recorded for this market."
}

// MatchesRule reports whether a signal event satisfies an alert rule.
//
// The checks are ordered cheapest-first and are purely logical: enablement,
// then scope, then per-field equality, then minimum severity, then per-signal
// numeric thresholds. There is no scoring, no fuzzy matching and no model
// inference, so a user can reproduce the outcome by reading the rule.
//
// A threshold only applies to the signal type it is defined for: a probability
// threshold is ignored for liquidity events and vice versa. This keeps a
// single rule usable across mixed event streams. When a threshold does apply
// but the event carries no measurement for it, the rule does not match:
// absence of evidence is not evidence of exceeding a threshold.
func MatchesRule(rule AlertRule, event SignalEvent, marketInWatchlist bool) bool {
	if !rule.Enabled {
		return false
	}
	if rule.WatchlistID != nil && !marketInWatchlist {
		return false
	}
	if rule.MarketID != nil && *rule.MarketID != event.MarketID {
		return false
	}
	if rule.SignalType != nil && *rule.SignalType != event.SignalType {
		return false
	}
	if rule.MinimumSeverity != nil && SeverityRank(event.Severity) < SeverityRank(*rule.MinimumSeverity) {
		return false
	}
	if rule.ProbabilityChangeThreshold != nil && event.SignalType == SignalTypeProbabilityShift {
		if !meetsThreshold(event.PercentagePoints, *rule.ProbabilityChangeThreshold) {
			return false
		}
	}
	if rule.ActivityChangeThreshold != nil && event.SignalType == SignalTypeActivityChange {
		if !meetsThreshold(event.PercentageChange, *rule.ActivityChangeThreshold) {
			return false
		}
	}
	if rule.LiquidityChangeThreshold != nil && event.SignalType == SignalTypeLiquidityChange {
		if !meetsThreshold(event.PercentageChange, *rule.LiquidityChangeThreshold) {
			return false
		}
	}
	return true
}

// meetsThreshold reports whether the absolute value of a measured change meets
// or exceeds a threshold. Thresholds compare on magnitude, so a large move in
// either direction triggers the alert. A missing or unparseable measurement
// never meets the threshold. Comparison is exact rational, so a change exactly
// equal to the threshold matches.
func meetsThreshold(measured *string, threshold string) bool {
	value, ok := parseDecimalRat(measured)
	if !ok {
		return false
	}
	limit, ok := parseDecimalRat(&threshold)
	if !ok {
		return false
	}
	return new(big.Rat).Abs(value).Cmp(new(big.Rat).Abs(limit)) >= 0
}

// ValidateAlertRule checks a user-supplied alert rule before it is persisted,
// returning a descriptive error naming the offending field. All checks are
// deterministic; a rule that passes validation here is accepted again on the
// next call with identical input.
func ValidateAlertRule(rule AlertRule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return errors.New("alert rule name must not be empty")
	}
	if len([]rune(rule.Name)) > maxAlertRuleNameLength {
		return fmt.Errorf("alert rule name must be at most %d characters", maxAlertRuleNameLength)
	}
	if rule.CooldownSeconds < 0 {
		return errors.New("cooldown_seconds must not be negative")
	}
	if rule.CooldownSeconds > maxAlertCooldownSeconds {
		return fmt.Errorf("cooldown_seconds must be at most %d seconds (7 days)", maxAlertCooldownSeconds)
	}
	if rule.SignalType != nil && !validSignalTypes[*rule.SignalType] {
		return fmt.Errorf("signal_type %q must be one of %s", *rule.SignalType, joinSignalTypes())
	}
	if rule.MinimumSeverity != nil && !isValidSeverity(*rule.MinimumSeverity) {
		return fmt.Errorf("minimum_severity %q must be one of %s", *rule.MinimumSeverity, strings.Join(validSeverities, ", "))
	}
	if err := validatePositiveThreshold("probability_change_threshold", rule.ProbabilityChangeThreshold); err != nil {
		return err
	}
	if err := validatePositiveThreshold("activity_change_threshold", rule.ActivityChangeThreshold); err != nil {
		return err
	}
	return validatePositiveThreshold("liquidity_change_threshold", rule.LiquidityChangeThreshold)
}

// validatePositiveThreshold requires a threshold to be a parseable decimal
// string strictly greater than zero.
func validatePositiveThreshold(field string, threshold *string) error {
	if threshold == nil {
		return nil
	}
	value := *threshold
	if _, err := validateDecimal(value, false); err != nil {
		return fmt.Errorf("%s must be a decimal string: %w", field, err)
	}
	number, ok := new(big.Rat).SetString(value)
	if !ok {
		return fmt.Errorf("%s must be a decimal string", field)
	}
	if number.Sign() <= 0 {
		return fmt.Errorf("%s must be greater than zero", field)
	}
	return nil
}

// SeverityDistribution counts events per severity. The returned map always
// contains all four severity keys, including those with a zero count, so that
// callers can render a complete breakdown without nil checks. Severities
// outside the known set are ignored so the shape of the result stays fixed.
func SeverityDistribution(events []SignalEvent) map[string]int {
	distribution := make(map[string]int, len(validSeverities))
	for _, severity := range validSeverities {
		distribution[severity] = 0
	}
	for _, event := range events {
		if _, known := distribution[event.Severity]; !known {
			continue
		}
		distribution[event.Severity]++
	}
	return distribution
}

// isValidSeverity reports whether severity is one of the four known levels.
func isValidSeverity(severity string) bool {
	for _, candidate := range validSeverities {
		if candidate == severity {
			return true
		}
	}
	return false
}

// joinSignalTypes renders the valid signal types in a fixed order for error
// messages, so the text never varies between calls.
func joinSignalTypes() string {
	return strings.Join([]string{
		SignalTypeNewMarket,
		SignalTypeProbabilityShift,
		SignalTypeActivityChange,
		SignalTypeLiquidityChange,
		SignalTypeMarketMovement,
	}, ", ")
}

// parseDecimalRat parses a nullable decimal string into an exact rational. It
// reports false when the pointer is nil or the text is not a valid decimal,
// which callers treat as "this value is not available".
func parseDecimalRat(value *string) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	number, ok := new(big.Rat).SetString(*value)
	if !ok {
		return nil, false
	}
	return number, true
}

// formatPercent renders a ratio as a human-facing percentage with one decimal
// place, e.g. 0.412 becomes "41.2%". Scaling and rounding are performed
// entirely in exact rational arithmetic.
func formatPercent(value *big.Rat) string {
	return new(big.Rat).Mul(value, hundred).FloatString(1) + "%"
}

// formatSignedPoints renders a signed ratio as percentage points with one
// decimal place and an explicit sign, e.g. 0.166 becomes "+16.6". A negative
// value keeps its own minus sign rather than gaining a second one.
func formatSignedPoints(value *big.Rat) string {
	scaled := new(big.Rat).Mul(value, hundred)
	formatted := scaled.FloatString(1)
	if scaled.Sign() >= 0 {
		return "+" + formatted
	}
	return formatted
}

// formatMagnitudePercent renders the unsigned magnitude of a ratio as a
// percentage with no decimal places, e.g. -0.38 becomes "38%".
func formatMagnitudePercent(value *big.Rat) string {
	return new(big.Rat).Abs(new(big.Rat).Mul(value, hundred)).FloatString(0) + "%"
}

// derefOrEmpty returns the pointed-to string, or the empty string when the
// pointer is nil. It keeps fingerprinting and sentence assembly free of nil
// checks without ever inventing a value: a missing component simply
// contributes nothing.
func derefOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
