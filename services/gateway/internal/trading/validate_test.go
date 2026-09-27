package trading

import (
	"testing"
)

func TestValidateQuoteInput(t *testing.T) {
	tests := []struct {
		name    string
		market  string
		side    string
		amount  string
		wallet  string
		wantErr bool
	}{
		{"valid", "m1", "YES", "10.50", "7xWallet", false},
		{"valid NO", "m1", "NO", "0.01", "7xWallet", false},
		{"missing market", "", "YES", "10", "7xWallet", true},
		{"invalid side", "m1", "MAYBE", "10", "7xWallet", true},
		{"missing side", "m1", "", "10", "7xWallet", true},
		{"invalid amount", "m1", "YES", "abc", "7xWallet", true},
		{"negative amount", "m1", "YES", "-5", "7xWallet", true},
		{"zero amount", "m1", "YES", "0", "7xWallet", true},
		{"zero decimal amount", "m1", "YES", "0.00", "7xWallet", true},
		{"missing wallet", "m1", "YES", "10", "", true},
		{"double decimal", "m1", "YES", "1.2.3", "7xWallet", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuoteInput(tt.market, tt.side, tt.amount, tt.wallet)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateBuildInput(t *testing.T) {
	if err := ValidateBuildInput("q1", "7xWallet", "7xWallet"); err != nil {
		t.Fatalf("expected valid build, got %v", err)
	}
	if err := ValidateBuildInput("", "7xWallet", "7xWallet"); err == nil {
		t.Fatal("expected error for empty quote reference")
	}
	if err := ValidateBuildInput("q1", "", "7xWallet"); err == nil {
		t.Fatal("expected error for empty wallet")
	}
	if err := ValidateBuildInput("q1", "7xWalletA", "7xWalletB"); err != ErrWalletMismatch {
		t.Fatalf("expected wallet mismatch, got %v", err)
	}
}

func TestValidateTransactionContext(t *testing.T) {
	// All matching → valid
	err := ValidateTransactionContext(
		"7xWallet", "7xWallet",
		"mainnet-beta", "mainnet-beta",
		"m1", "m1",
		"YES", "YES",
		"10.50", "10.50",
	)
	if err != nil {
		t.Fatalf("expected valid context, got %v", err)
	}

	// Wallet mismatch
	err = ValidateTransactionContext("7xWalletA", "7xWalletB", "mainnet-beta", "mainnet-beta", "m1", "m1", "YES", "YES", "10", "10")
	if err != ErrWalletMismatch {
		t.Fatalf("expected wallet mismatch, got %v", err)
	}

	// Network mismatch
	err = ValidateTransactionContext("7xWallet", "7xWallet", "mainnet-beta", "devnet", "m1", "m1", "YES", "YES", "10", "10")
	if err != ErrNetworkMismatch {
		t.Fatalf("expected network mismatch, got %v", err)
	}

	// Market mismatch
	err = ValidateTransactionContext("7xWallet", "7xWallet", "mainnet-beta", "mainnet-beta", "m1", "m2", "YES", "YES", "10", "10")
	if err != ErrMarketMismatch {
		t.Fatalf("expected market mismatch, got %v", err)
	}

	// Side mismatch
	err = ValidateTransactionContext("7xWallet", "7xWallet", "mainnet-beta", "mainnet-beta", "m1", "m1", "YES", "NO", "10", "10")
	if err != ErrSideMismatch {
		t.Fatalf("expected side mismatch, got %v", err)
	}

	// Amount mismatch
	err = ValidateTransactionContext("7xWallet", "7xWallet", "mainnet-beta", "mainnet-beta", "m1", "m1", "YES", "YES", "10", "20")
	if err != ErrAmountMismatch {
		t.Fatalf("expected amount mismatch, got %v", err)
	}
}

func TestValidateBroadcastAndReport(t *testing.T) {
	if err := ValidateBroadcastInput("a1", "signed", "7xWallet"); err != nil {
		t.Fatalf("expected valid broadcast, got %v", err)
	}
	if err := ValidateBroadcastInput("", "signed", "7xWallet"); err == nil {
		t.Fatal("expected error for empty attempt")
	}
	if err := ValidateBroadcastInput("a1", "", "7xWallet"); err == nil {
		t.Fatal("expected error for empty signed tx")
	}
	if err := ValidateReportInput("a1", "sig"); err != nil {
		t.Fatalf("expected valid report, got %v", err)
	}
	if err := ValidateReportInput("", "sig"); err == nil {
		t.Fatal("expected error for empty attempt")
	}
}

func TestNormalizeDecimal(t *testing.T) {
	cases := map[string]string{
		"10.50":  "10.5",
		"10.5":   "10.5",
		"0.10":   "0.1",
		"0.1":    "0.1",
		"10":     "10",
		"010":    "10",
		"100.00": "100",
	}
	for input, want := range cases {
		if got := normalizeDecimal(input); got != want {
			t.Errorf("normalizeDecimal(%q) = %q, want %q", input, got, want)
		}
	}
}
