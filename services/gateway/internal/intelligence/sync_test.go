package intelligence

import "testing"

func TestObservationsEqualUsesExactDecimalValuesAndNulls(t *testing.T) {
	previous := Observation{YesProbability: stringValue("0.630000"), VolumeUSDC: stringValue("100.00")}
	current := Observation{YesProbability: stringValue("0.63"), VolumeUSDC: stringValue("100")}
	equal, err := observationsEqual(previous, current)
	if err != nil || !equal {
		t.Fatalf("equal=%t err=%v", equal, err)
	}
	current.VolumeUSDC = nil
	equal, err = observationsEqual(previous, current)
	if err != nil || equal {
		t.Fatalf("null difference equal=%t err=%v", equal, err)
	}
	previous.YesProbability = stringValue("invalid")
	if _, err := observationsEqual(previous, current); err == nil {
		t.Fatal("expected malformed stored decimal to fail")
	}
}

func stringValue(value string) *string { return &value }
