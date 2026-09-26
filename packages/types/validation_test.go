package types

import "testing"

func TestIsBase58Address(t *testing.T) {
	if !IsBase58Address("11111111111111111111111111111111") {
		t.Fatal("expected the zero public key to be valid base58")
	}
	for _, value := range []string{"", "Mkt111", "0OIl", "1111111111111111111111111111111/"} {
		if IsBase58Address(value) {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}
