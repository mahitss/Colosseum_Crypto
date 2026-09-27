package trading

import (
	"errors"
	"strings"
)

// VALIDATION
//
// Before a wallet is asked to sign, the backend validates that the transaction
// context matches the UI state the user actually reviewed. We never trust
// client-supplied confirmation values or arbitrary transaction bytes from the
// browser — the transaction comes from Panta, and we validate its expected
// wallnet/network/market/side/amount against the user's request.

var (
	ErrInvalidAmount             = errors.New("amount must be a positive decimal string")
	ErrInvalidSide               = errors.New("side must be YES or NO")
	ErrWalletRequired            = errors.New("wallet public key is required")
	ErrMarketRequired            = errors.New("market identifier is required")
	ErrWalletMismatch            = errors.New("transaction wallet does not match connected wallet")
	ErrNetworkMismatch           = errors.New("transaction network does not match expected network")
	ErrMarketMismatch            = errors.New("transaction market does not match requested market")
	ErrSideMismatch              = errors.New("transaction side does not match requested side")
	ErrAmountMismatch            = errors.New("transaction amount does not match requested amount")
	ErrTradeAttemptRequired      = errors.New("trade attempt identifier is required")
	ErrSignatureRequired         = errors.New("transaction signature is required")
	ErrSignedTransactionRequired = errors.New("signed transaction is required")
)

// ValidateQuoteInput validates the user's quote request before calling Panta.
func ValidateQuoteInput(marketID, side, amountUSDC, walletPubkey string) error {
	if strings.TrimSpace(marketID) == "" {
		return ErrMarketRequired
	}
	if side != "YES" && side != "NO" {
		return ErrInvalidSide
	}
	if !validDecimal(amountUSDC) || !positiveDecimal(amountUSDC) {
		return ErrInvalidAmount
	}
	if strings.TrimSpace(walletPubkey) == "" {
		return ErrWalletRequired
	}
	return nil
}

// ValidateBuildInput validates a build request against the stored quote context.
func ValidateBuildInput(quoteReference, walletPubkey, expectedWallet string) error {
	if strings.TrimSpace(quoteReference) == "" {
		return ErrTradeAttemptRequired
	}
	if strings.TrimSpace(walletPubkey) == "" {
		return ErrWalletRequired
	}
	if expectedWallet != "" && expectedWallet != walletPubkey {
		return ErrWalletMismatch
	}
	return nil
}

// ValidateTransactionContext ensures the built transaction matches the user's
// reviewed request (wallet, network, market, side, amount).
func ValidateTransactionContext(
	expectedWallet, connectedWallet string,
	expectedNetwork, actualNetwork string,
	requestedMarketID, transactionMarketID string,
	requestedSide, transactionSide string,
	requestedAmount, transactionAmount string,
) error {
	if expectedWallet != "" && expectedWallet != connectedWallet {
		return ErrWalletMismatch
	}
	if actualNetwork != "" && expectedNetwork != "" && actualNetwork != expectedNetwork {
		return ErrNetworkMismatch
	}
	if requestedMarketID != "" && transactionMarketID != "" && requestedMarketID != transactionMarketID {
		return ErrMarketMismatch
	}
	if requestedSide != "" && transactionSide != "" && requestedSide != transactionSide {
		return ErrSideMismatch
	}
	if requestedAmount != "" && transactionAmount != "" && requestedAmount != transactionAmount {
		return ErrAmountMismatch
	}
	return nil
}

// ValidateBroadcastInput validates the client's broadcast request.
func ValidateBroadcastInput(tradeAttemptID, signedTx, walletPubkey string) error {
	if strings.TrimSpace(tradeAttemptID) == "" {
		return ErrTradeAttemptRequired
	}
	if strings.TrimSpace(signedTx) == "" {
		return ErrSignedTransactionRequired
	}
	if strings.TrimSpace(walletPubkey) == "" {
		return ErrWalletRequired
	}
	return nil
}

// ValidateReportInput validates a report request.
func ValidateReportInput(tradeAttemptID, signature string) error {
	if strings.TrimSpace(tradeAttemptID) == "" {
		return ErrTradeAttemptRequired
	}
	if strings.TrimSpace(signature) == "" {
		return ErrSignatureRequired
	}
	return nil
}

func validDecimal(s string) bool {
	if s == "" {
		return false
	}
	hasDot := false
	for _, ch := range s {
		if ch == '.' {
			if hasDot {
				return false
			}
			hasDot = true
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// positiveDecimal returns true when the decimal string represents a value > 0.
// It compares using integer arithmetic on the digits to avoid float conversion,
// consistent with the "never use float64 for user money" rule.
func positiveDecimal(s string) bool {
	integer := s
	fraction := ""
	if dot := strings.Index(s, "."); dot >= 0 {
		integer = s[:dot]
		fraction = s[dot+1:]
	}
	// A value is non-positive when every digit is zero.
	allZero := true
	for _, ch := range integer + fraction {
		if ch != '0' {
			allZero = false
			break
		}
	}
	return !allZero
}

// normalizeDecimal strips leading zeros and trailing zeros after the decimal
// point so that "10.50" and "10.5" compare equal.
func normalizeDecimal(s string) string {
	if !strings.Contains(s, ".") {
		return strings.TrimLeft(s, "0")
	}
	integer := s[:strings.Index(s, ".")]
	fraction := s[strings.Index(s, ".")+1:]
	integer = strings.TrimLeft(integer, "0")
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return integer
	}
	if integer == "" {
		integer = "0"
	}
	return integer + "." + fraction
}
