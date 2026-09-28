package intelligence

import (
	"strings"
	"testing"
	"time"
)

// ptr returns a pointer to value. Tests in this package use it to express the
// nullable decimal fields on SignalEvent and AlertRule inline.
func ptr(value string) *string { return &value }

// baseEvent is a valid PROBABILITY_SHIFT event that satisfies baseRule.
func baseEvent() SignalEvent {
	return SignalEvent{
		ID:               77,
		MarketID:         "3f0b6c1e-1111-4a2b-9c3d-abcdefabcdef",
		SourceMarketID:   "panta-42",
		SignalType:       SignalTypeProbabilityShift,
		Severity:         SeveritySignificant,
		Metric:           "yes_probability",
		PreviousValue:    ptr("0.412"),
		CurrentValue:     ptr("0.578"),
		PercentagePoints: ptr("0.166"),
		ObservationID:    991,
		ObservedAt:       time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC),
		CreatedAt:        time.Date(2026, time.September, 1, 12, 0, 5, 0, time.UTC),
		Source:           SourcePanta,
	}
}

// baseRule is a valid rule that matches baseEvent.
func baseRule() AlertRule {
	return AlertRule{
		ID:              "7c1d2e3f-4444-4a5b-8c9d-0123456789ab",
		UserID:          "user-1",
		Name:            "Probability shift alerts",
		Enabled:         true,
		MinimumSeverity: ptr(SeveritySignificant),
		CooldownSeconds: 3600,
	}
}

// isHex64 reports whether value is exactly 64 lowercase hex characters, the
// shape every fingerprint and dedupe key must have.
func isHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// TestSignalFingerprintIsDeterministicAndDistinct pins the two properties the
// dedupe layer depends on: the fingerprint is a pure function of the observed
// fact, and it changes whenever that fact changes.
func TestSignalFingerprintIsDeterministicAndDistinct(t *testing.T) {
	baseFingerprint := SignalFingerprint(baseEvent())

	if again := SignalFingerprint(baseEvent()); again != baseFingerprint {
		t.Errorf("SignalFingerprint is not deterministic: first %q, second %q", baseFingerprint, again)
	}
	if !isHex64(baseFingerprint) {
		t.Errorf("fingerprint %q is not 64 lowercase hex characters", baseFingerprint)
	}

	t.Run("distinct", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*SignalEvent)
		}{
			{"MarketID", func(e *SignalEvent) { e.MarketID = "00000000-0000-4000-8000-000000000000" }},
			{"SignalType", func(e *SignalEvent) { e.SignalType = SignalTypeActivityChange }},
			{"ObservationID", func(e *SignalEvent) { e.ObservationID = 992 }},
			{"Metric", func(e *SignalEvent) { e.Metric = "volume_usdc" }},
			{"CurrentValue", func(e *SignalEvent) { e.CurrentValue = ptr("0.579") }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				event := baseEvent()
				tc.mutate(&event)
				if got := SignalFingerprint(event); got == baseFingerprint {
					t.Errorf("changing %s left the fingerprint unchanged (%q)", tc.name, got)
				}
			})
		}
	})

	t.Run("database id excluded", func(t *testing.T) {
		unsaved := baseEvent()
		unsaved.ID = 0
		if got := SignalFingerprint(unsaved); got != baseFingerprint {
			t.Errorf("ID=0 fingerprint %q differs from persisted fingerprint %q; re-detection would duplicate", got, baseFingerprint)
		}
	})

	t.Run("timestamps excluded", func(t *testing.T) {
		relater := baseEvent()
		relater.CreatedAt = time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
		relater.ObservedAt = time.Date(1999, time.December, 31, 23, 59, 59, 0, time.UTC)
		if got := SignalFingerprint(relater); got != baseFingerprint {
			t.Errorf("timestamp change altered fingerprint: got %q, want %q", got, baseFingerprint)
		}
	})
}

// TestSeverityRankOrdering checks the ranking that minimum-severity filters
// depend on, including that unrecognised input never outranks a real level.
func TestSeverityRankOrdering(t *testing.T) {
	ranks := []struct {
		severity string
		want     int
	}{
		{SeverityInfo, 1},
		{SeverityWatch, 2},
		{SeveritySignificant, 3},
		{SeverityCritical, 4},
		{"", 0},
		{"bogus", 0},
	}
	for _, tc := range ranks {
		if got := SeverityRank(tc.severity); got != tc.want {
			t.Errorf("SeverityRank(%q) = %d, want %d", tc.severity, got, tc.want)
		}
	}

	critical := SeverityRank(SeverityCritical)
	significant := SeverityRank(SeveritySignificant)
	watch := SeverityRank(SeverityWatch)
	info := SeverityRank(SeverityInfo)
	bogus := SeverityRank("bogus")
	if !(critical > significant && significant > watch && watch > info && info > bogus) {
		t.Errorf("severity ordering violated: CRITICAL=%d SIGNIFICANT=%d WATCH=%d INFO=%d bogus=%d",
			critical, significant, watch, info, bogus)
	}
}

// TestExplainSignalNeverInventsValues asserts the exact rendered sentences, so
// a missing field can never be silently filled in with a guess.
func TestExplainSignalNeverInventsValues(t *testing.T) {
	cases := []struct {
		name     string
		event    SignalEvent
		want     string
		contains []string
		absent   []string
	}{
		{
			name: "new market",
			event: SignalEvent{
				MarketID:   "market-1",
				SignalType: SignalTypeNewMarket,
				Severity:   SeverityInfo,
				Metric:     "existence",
			},
			want: "New market discovered in the Prophet catalog.",
		},
		{
			name: "probability shift reports percentage points",
			event: SignalEvent{
				MarketID:         "market-1",
				SignalType:       SignalTypeProbabilityShift,
				Severity:         SeveritySignificant,
				PreviousValue:    ptr("0.412"),
				CurrentValue:     ptr("0.578"),
				PercentagePoints: ptr("0.166"),
			},
			want: "YES probability moved from 41.2% to 57.8% (+16.6 percentage points).",
		},
		{
			name: "activity increase with both endpoints",
			event: SignalEvent{
				MarketID:         "market-1",
				SignalType:       SignalTypeActivityChange,
				Severity:         SeverityWatch,
				PercentageChange: ptr("0.38"),
				PreviousValue:    ptr("100"),
				CurrentValue:     ptr("138"),
			},
			contains: []string{"Activity increased 38%"},
		},
		{
			name: "activity decrease",
			event: SignalEvent{
				MarketID:         "market-1",
				SignalType:       SignalTypeActivityChange,
				Severity:         SeverityWatch,
				PercentageChange: ptr("-0.12"),
			},
			contains: []string{"Activity decreased 12%"},
		},
		{
			name: "no measurements at all",
			event: SignalEvent{
				MarketID:   "market-1",
				SignalType: SignalTypeProbabilityShift,
				Severity:   SeverityWatch,
				Metric:     "yes_probability",
			},
			want: "New observation recorded for this market.",
		},
		{
			name: "current value only",
			event: SignalEvent{
				MarketID:     "market-1",
				SignalType:   SignalTypeProbabilityShift,
				Severity:     SeverityWatch,
				CurrentValue: ptr("0.5"),
			},
			want: "Current value for this market is 0.5.",
		},
		{
			name: "percentage points without a previous value does not imply movement",
			event: SignalEvent{
				MarketID:         "market-1",
				SignalType:       SignalTypeProbabilityShift,
				Severity:         SeverityWatch,
				PercentagePoints: ptr("0.166"),
			},
			want:   "Signal recorded for this market.",
			absent: []string{"moved from"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExplainSignal(tc.event)
			if tc.want != "" && got != tc.want {
				t.Errorf("ExplainSignal = %q, want %q", got, tc.want)
			}
			for _, needle := range tc.contains {
				if !strings.Contains(got, needle) {
					t.Errorf("ExplainSignal = %q, want it to contain %q", got, needle)
				}
			}
			for _, needle := range tc.absent {
				if strings.Contains(got, needle) {
					t.Errorf("ExplainSignal = %q, want it to NOT contain %q", got, needle)
				}
			}
		})
	}
}

// TestMatchesRuleRespectsEveryCondition walks one predicate at a time, so a
// failure names the exact condition that regressed.
func TestMatchesRuleRespectsEveryCondition(t *testing.T) {
	base := baseEvent()
	otherMarket := "99999999-9999-4999-8999-999999999999"

	cases := []struct {
		name              string
		mutateRule        func(*AlertRule)
		event             SignalEvent
		marketInWatchlist bool
		want              bool
	}{
		{
			name: "base rule matches base event",
			want: true,
		},
		{
			name:       "disabled rule never matches",
			mutateRule: func(r *AlertRule) { r.Enabled = false },
			want:       false,
		},
		{
			name:       "minimum severity above event severity",
			mutateRule: func(r *AlertRule) { r.MinimumSeverity = ptr(SeverityCritical) },
			want:       false,
		},
		{
			name:       "minimum severity below event severity",
			mutateRule: func(r *AlertRule) { r.MinimumSeverity = ptr(SeverityWatch) },
			want:       true,
		},
		{
			name:       "signal type mismatch",
			mutateRule: func(r *AlertRule) { r.SignalType = ptr(SignalTypeLiquidityChange) },
			want:       false,
		},
		{
			name:       "signal type match",
			mutateRule: func(r *AlertRule) { r.SignalType = ptr(SignalTypeProbabilityShift) },
			want:       true,
		},
		{
			name:       "scoped to a different market",
			mutateRule: func(r *AlertRule) { r.MarketID = &otherMarket },
			want:       false,
		},
		{
			name:       "scoped to this market",
			mutateRule: func(r *AlertRule) { r.MarketID = ptr(base.MarketID) },
			want:       true,
		},
		{
			name:              "watchlist scoped, market absent",
			mutateRule:        func(r *AlertRule) { r.WatchlistID = ptr("watchlist-1") },
			marketInWatchlist: false,
			want:              false,
		},
		{
			name:              "watchlist scoped, market present",
			mutateRule:        func(r *AlertRule) { r.WatchlistID = ptr("watchlist-1") },
			marketInWatchlist: true,
			want:              true,
		},
		{
			name:       "probability threshold met",
			mutateRule: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.10") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeProbabilityShift,
				Severity: SeveritySignificant, PercentagePoints: ptr("0.16"),
			},
			want: true,
		},
		{
			name:       "probability threshold not met",
			mutateRule: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.10") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeProbabilityShift,
				Severity: SeveritySignificant, PercentagePoints: ptr("0.04"),
			},
			want: false,
		},
		{
			name:       "probability threshold with no measurement",
			mutateRule: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.10") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeProbabilityShift,
				Severity: SeveritySignificant,
			},
			want: false,
		},
		{
			name:       "probability threshold at exact boundary",
			mutateRule: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.166") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeProbabilityShift,
				Severity: SeveritySignificant, PercentagePoints: ptr("0.166"),
			},
			want: true,
		},
		{
			name:       "probability threshold ignored for activity change",
			mutateRule: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.10") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeActivityChange,
				Severity: SeveritySignificant,
			},
			want: true,
		},
		{
			name:       "activity threshold uses magnitude not direction",
			mutateRule: func(r *AlertRule) { r.ActivityChangeThreshold = ptr("0.25") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeActivityChange,
				Severity: SeveritySignificant, PercentageChange: ptr("-0.30"),
			},
			want: true,
		},
		{
			name:       "liquidity threshold not met",
			mutateRule: func(r *AlertRule) { r.LiquidityChangeThreshold = ptr("0.25") },
			event: SignalEvent{
				MarketID: base.MarketID, SignalType: SignalTypeLiquidityChange,
				Severity: SeveritySignificant, PercentageChange: ptr("0.20"),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := baseRule()
			if tc.mutateRule != nil {
				tc.mutateRule(&rule)
			}
			event := base
			if tc.event.SignalType != "" {
				event = tc.event
			}
			if got := MatchesRule(rule, event, tc.marketInWatchlist); got != tc.want {
				t.Errorf("MatchesRule = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestValidateAlertRule covers the user-facing rejection rules.
func TestValidateAlertRule(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*AlertRule)
		wantErr bool
	}{
		{name: "empty name", mutate: func(r *AlertRule) { r.Name = "" }, wantErr: true},
		{name: "blank name", mutate: func(r *AlertRule) { r.Name = "   " }, wantErr: true},
		{name: "name of 121 runes", mutate: func(r *AlertRule) { r.Name = strings.Repeat("a", 121) }, wantErr: true},
		{name: "name of 120 runes", mutate: func(r *AlertRule) { r.Name = strings.Repeat("a", 120) }},
		{name: "negative cooldown", mutate: func(r *AlertRule) { r.CooldownSeconds = -1 }, wantErr: true},
		{name: "cooldown above seven days", mutate: func(r *AlertRule) { r.CooldownSeconds = 604801 }, wantErr: true},
		{name: "cooldown of exactly seven days", mutate: func(r *AlertRule) { r.CooldownSeconds = 604800 }},
		{name: "unknown signal type", mutate: func(r *AlertRule) { r.SignalType = ptr("WEATHER_CHANGE") }, wantErr: true},
		{name: "unknown minimum severity", mutate: func(r *AlertRule) { r.MinimumSeverity = ptr("APOCALYPTIC") }, wantErr: true},
		{name: "zero threshold", mutate: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0") }, wantErr: true},
		{name: "negative threshold", mutate: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("-1") }, wantErr: true},
		{name: "non-numeric threshold", mutate: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("abc") }, wantErr: true},
		{name: "valid probability threshold", mutate: func(r *AlertRule) { r.ProbabilityChangeThreshold = ptr("0.10") }},
		{name: "valid activity threshold", mutate: func(r *AlertRule) { r.ActivityChangeThreshold = ptr("0.25") }},
		{name: "invalid liquidity threshold", mutate: func(r *AlertRule) { r.LiquidityChangeThreshold = ptr("0") }, wantErr: true},
		{
			name: "fully valid rule",
			mutate: func(r *AlertRule) {
				r.SignalType = ptr(SignalTypeProbabilityShift)
				r.MinimumSeverity = ptr(SeverityWatch)
				r.ProbabilityChangeThreshold = ptr("0.10")
				r.ActivityChangeThreshold = ptr("0.25")
				r.LiquidityChangeThreshold = ptr("0.5")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := baseRule()
			tc.mutate(&rule)
			err := ValidateAlertRule(rule)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateAlertRule returned nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateAlertRule returned %v, want nil", err)
			}
		})
	}
}

// TestSeverityDistributionAlwaysHasAllKeys ensures the map shape is fixed so
// callers never have to nil-check a missing bucket.
func TestSeverityDistributionAlwaysHasAllKeys(t *testing.T) {
	assertShape := func(t *testing.T, got map[string]int) {
		t.Helper()
		for _, severity := range validSeverities {
			if _, ok := got[severity]; !ok {
				t.Errorf("severity distribution is missing key %q", severity)
			}
		}
	}

	t.Run("empty input", func(t *testing.T) {
		got := SeverityDistribution(nil)
		assertShape(t, got)
		for _, severity := range validSeverities {
			if got[severity] != 0 {
				t.Errorf("SeverityDistribution(nil)[%q] = %d, want 0", severity, got[severity])
			}
		}
		if len(got) != 4 {
			t.Errorf("SeverityDistribution(nil) has %d keys, want 4", len(got))
		}
	})

	t.Run("mixed input", func(t *testing.T) {
		events := []SignalEvent{
			{Severity: SeverityCritical},
			{Severity: SeverityInfo},
			{Severity: SeverityInfo},
			{Severity: SeverityWatch},
		}
		got := SeverityDistribution(events)
		assertShape(t, got)
		want := map[string]int{
			SeverityInfo:        2,
			SeverityWatch:       1,
			SeveritySignificant: 0,
			SeverityCritical:    1,
		}
		for severity, count := range want {
			if got[severity] != count {
				t.Errorf("SeverityDistribution[%q] = %d, want %d", severity, got[severity], count)
			}
		}
	})

	t.Run("unknown severity is ignored", func(t *testing.T) {
		events := []SignalEvent{
			{Severity: "URGENT"},
			{Severity: ""},
			{Severity: SeveritySignificant},
		}
		got := SeverityDistribution(events)
		assertShape(t, got)
		if got[SeveritySignificant] != 1 {
			t.Errorf("SeverityDistribution[SIGNIFICANT] = %d, want 1", got[SeveritySignificant])
		}
		if len(got) != 4 {
			t.Errorf("unknown severity added a key: %v", got)
		}
	})
}

// TestAlertDedupeKeyVariesByRuleAndEvent checks the suppression key separates
// distinct (rule, event) pairs while collapsing true repeats.
func TestAlertDedupeKeyVariesByRuleAndEvent(t *testing.T) {
	rule := baseRule()
	baseKey := AlertDedupeKey(rule.ID, baseEvent())

	if again := AlertDedupeKey(rule.ID, baseEvent()); again != baseKey {
		t.Errorf("AlertDedupeKey is not deterministic: first %q, second %q", baseKey, again)
	}
	if !isHex64(baseKey) {
		t.Errorf("dedupe key %q is not 64 lowercase hex characters", baseKey)
	}

	otherRule := baseRule()
	otherRule.ID = "00000000-0000-4000-8000-000000000000"
	if got := AlertDedupeKey(otherRule.ID, baseEvent()); got == baseKey {
		t.Errorf("different rule ID produced the same dedupe key %q", got)
	}

	otherEvent := baseEvent()
	otherEvent.ObservationID = 992
	if got := AlertDedupeKey(rule.ID, otherEvent); got == baseKey {
		t.Errorf("different event produced the same dedupe key %q", got)
	}
}
