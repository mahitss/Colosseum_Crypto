package trading

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PantaClient is the interface the service uses to talk to Panta.
// This keeps the service testable with httptest-backed fakes.
type PantaClient interface {
	QuotePrimaryBuy(ctx context.Context, marketID, side, amountUSDC, walletPubkey string) (PantaQuote, error)
	BuildPrimaryBuy(ctx context.Context, quoteReference, walletPubkey string) (PantaBuild, error)
	ReportTransaction(ctx context.Context, marketID, side, amountUSDC, signature, quoteReference string) (string, error)
	VerifyTransaction(ctx context.Context, signature string) (PantaVerification, error)
	GetPositions(ctx context.Context, walletPubkey string) ([]PantaPosition, error)
	GetAccount(ctx context.Context) (PantaAccount, error)
}

// PantaQuote mirrors the documented Panta quote schema.
type PantaQuote struct {
	MarketID       string `json:"market_id"`
	Side           string `json:"side"`
	AmountUSDC     string `json:"amount_usdc"`
	PricePerShare  string `json:"price_per_share"`
	TotalCost      string `json:"total_cost"`
	SharesReceived string `json:"shares_received"`
	QuoteReference string `json:"quote_reference"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

// PantaBuild mirrors the documented Panta build schema.
type PantaBuild struct {
	TransactionData string   `json:"transaction"`
	ExpectedWallet  string   `json:"expected_wallet"`
	ExpectedNetwork string   `json:"network"`
	BuildReference  string   `json:"build_reference"`
	MessageFormat   string   `json:"message_format"`
	Instructions    []string `json:"instructions,omitempty"`
}

// PantaVerification mirrors the Panta verification response.
type PantaVerification struct {
	Status  string `json:"status"` // verified | pending | failed
	Message string `json:"message,omitempty"`
}

// PantaPosition mirrors a Panta position record.
type PantaPosition struct {
	MarketID    string `json:"market_id"`
	MarketTitle string `json:"market_title,omitempty"`
	Side        string `json:"side"`
	Quantity    string `json:"quantity"`
	ValueUSDC   string `json:"value_usdc,omitempty"`
	Status      string `json:"status"`
}

// PantaAccount mirrors the Panta account response for authentication validation.
type PantaAccount struct {
	UserID           string `json:"userId"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	CanCreateMarkets bool   `json:"canCreateMarkets"`
	CreatedAt        string `json:"createdAt"`
	APIKeyID         string `json:"apiKeyId"`
}

// PantaHTTPClient is the production implementation of PantaClient.
type PantaHTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewPantaHTTPClient(baseURL, apiKey string, timeout time.Duration) (*PantaHTTPClient, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, errors.New("PANTA_API_URL must be an absolute HTTP(S) URL")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("PANTA_API_KEY is required for trading")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &PantaHTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/") + "/",
		apiKey:     apiKey,
		httpClient: httpClient,
	}, nil
}

func (c *PantaHTTPClient) doJSON(ctx context.Context, method, path string, body any, destination any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.New("request encode failed")
		}
		reader = strings.NewReader(string(raw))
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return errors.New("request build failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Api-Key", c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return errors.New("Panta unavailable")
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch response.StatusCode {
		case http.StatusUnauthorized:
			return errors.New("Panta authentication failed")
		case http.StatusTooManyRequests:
			return errors.New("Panta rate limit exceeded")
		case http.StatusBadRequest:
			return errors.New("Panta rejected request")
		default:
			return errors.New("Panta request failed")
		}
	}

	if destination == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(destination); err != nil {
		return errors.New("Panta returned malformed data")
	}
	return nil
}

func (c *PantaHTTPClient) QuotePrimaryBuy(ctx context.Context, marketID, side, amountUSDC, walletPubkey string) (PantaQuote, error) {
	var quote PantaQuote
	err := c.doJSON(ctx, http.MethodPost, "markets/primary-buy/quote/", map[string]any{
		"market_id":   marketID,
		"side":        side,
		"amount_usdc": amountUSDC,
		"wallet":      walletPubkey,
	}, &quote)
	if err != nil {
		return PantaQuote{}, err
	}
	if quote.QuoteReference == "" {
		return PantaQuote{}, errors.New("Panta quote missing reference")
	}
	return quote, nil
}

func (c *PantaHTTPClient) BuildPrimaryBuy(ctx context.Context, quoteReference, walletPubkey string) (PantaBuild, error) {
	var build PantaBuild
	err := c.doJSON(ctx, http.MethodPost, "markets/primary-buy/build/", map[string]any{
		"quote_reference": quoteReference,
		"wallet":          walletPubkey,
	}, &build)
	if err != nil {
		return PantaBuild{}, err
	}
	if build.TransactionData == "" {
		return PantaBuild{}, errors.New("Panta build returned no transaction")
	}
	return build, nil
}

func (c *PantaHTTPClient) ReportTransaction(ctx context.Context, marketID, side, amountUSDC, signature, quoteReference string) (string, error) {
	var result struct {
		Reference string `json:"order_reference"`
	}
	err := c.doJSON(ctx, http.MethodPost, "markets/primary-buy/report/", map[string]any{
		"market_id":       marketID,
		"side":            side,
		"amount_usdc":     amountUSDC,
		"signature":       signature,
		"quote_reference": quoteReference,
	}, &result)
	if err != nil {
		return "", err
	}
	return result.Reference, nil
}

func (c *PantaHTTPClient) VerifyTransaction(ctx context.Context, signature string) (PantaVerification, error) {
	var verification PantaVerification
	err := c.doJSON(ctx, http.MethodGet, "transactions/"+signature+"/verify/", nil, &verification)
	if err != nil {
		return PantaVerification{}, err
	}
	return verification, nil
}

func (c *PantaHTTPClient) GetPositions(ctx context.Context, walletPubkey string) ([]PantaPosition, error) {
	var result struct {
		Positions []PantaPosition `json:"positions"`
	}
	err := c.doJSON(ctx, http.MethodGet, "positions/?wallet="+url.QueryEscape(walletPubkey), nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Positions, nil
}

func (c *PantaHTTPClient) GetAccount(ctx context.Context) (PantaAccount, error) {
	var account PantaAccount
	err := c.doJSON(ctx, http.MethodGet, "account/", nil, &account)
	if err != nil {
		return PantaAccount{}, err
	}
	if account.UserID == "" || account.Status == "" {
		return PantaAccount{}, errors.New("Panta account response malformed")
	}
	return account, nil
}

// Service orchestrates the full trade lifecycle.
type Service struct {
	store      Store
	pool       *pgxpool.Pool
	panta      PantaClient
	broadcast  *Broadcaster
	confirmLvl ConfirmationLevel
	timeout    time.Duration
}

func NewService(store Store, pool *pgxpool.Pool, panta PantaClient, broadcast *Broadcaster) *Service {
	return &Service{
		store:      store,
		pool:       pool,
		panta:      panta,
		broadcast:  broadcast,
		confirmLvl: ConfirmationConfirmed,
		timeout:    60 * time.Second,
	}
}

func (s *Service) Quote(ctx context.Context, marketID, side, amountUSDC, walletPubkey string) (PantaQuote, *Attempt, error) {
	if err := ValidateQuoteInput(marketID, side, amountUSDC, walletPubkey); err != nil {
		return PantaQuote{}, nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PantaQuote{}, nil, errors.New("storage unavailable")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	attemptID, err := s.store.CreateAttempt(ctx, tx, Attempt{
		WalletAddress: walletPubkey,
		MarketID:      marketID,
		Side:          side,
		AmountUSDC:    amountUSDC,
		Status:        StatusQuoting,
	})
	if err != nil {
		return PantaQuote{}, nil, errors.New("could not record trade attempt")
	}
	if err := tx.Commit(ctx); err != nil {
		return PantaQuote{}, nil, err
	}

	// Quote is read-only against Panta.
	quote, err := s.panta.QuotePrimaryBuy(ctx, marketID, side, amountUSDC, walletPubkey)
	if err != nil {
		s.fail(ctx, attemptID, ErrorQuoteFailed, err.Error())
		return PantaQuote{}, nil, err
	}

	// Persist quote reference and move to QUOTE_READY.
	tx2, err := s.pool.Begin(ctx)
	if err != nil {
		return quote, nil, err
	}
	defer tx2.Rollback(ctx) //nolint:errcheck

	if err := s.attachQuoteRef(ctx, tx2, attemptID, quote.QuoteReference); err != nil {
		return quote, nil, err
	}
	if err := tx2.Commit(ctx); err != nil {
		return quote, nil, err
	}

	attempt := &Attempt{
		ID:             attemptID,
		WalletAddress:  walletPubkey,
		MarketID:       marketID,
		Side:           side,
		AmountUSDC:     amountUSDC,
		QuoteReference: quote.QuoteReference,
		Status:         StatusQuoteReady,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return quote, attempt, nil
}

func (s *Service) attachQuoteRef(ctx context.Context, tx pgx.Tx, id, reference string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE trade_attempts SET quote_reference = $2, status = 'QUOTE_READY', updated_at = now()
		WHERE id = $1::uuid`, id, reference); err != nil {
		return err
	}
	return nil
}

func (s *Service) Build(ctx context.Context, quoteReference, walletPubkey string) (PantaBuild, error) {
	build, err := s.panta.BuildPrimaryBuy(ctx, quoteReference, walletPubkey)
	if err != nil {
		return PantaBuild{}, err
	}
	if err := ValidateBuildInput(quoteReference, walletPubkey, build.ExpectedWallet); err != nil {
		return PantaBuild{}, err
	}
	return build, nil
}

func (s *Service) Broadcast(ctx context.Context, tradeAttemptID, signedTx, walletPubkey string) (BroadcastResult, error) {
	if err := ValidateBroadcastInput(tradeAttemptID, signedTx, walletPubkey); err != nil {
		return BroadcastResult{}, err
	}

	attempt, err := s.getAttempt(ctx, tradeAttemptID)
	if err != nil {
		return BroadcastResult{}, err
	}
	if attempt == nil {
		return BroadcastResult{}, errors.New("trade attempt not found")
	}
	if attempt.WalletAddress != walletPubkey {
		return BroadcastResult{}, ErrWalletMismatch
	}

	if err := s.update(ctx, tradeAttemptID, StatusBroadcasting); err != nil {
		return BroadcastResult{}, err
	}

	result, err := s.broadcast.SendRawTransaction(ctx, signedTx)
	if err != nil {
		s.fail(ctx, tradeAttemptID, ErrorBroadcastFailed, err.Error())
		return BroadcastResult{}, err
	}

	if err := s.update(ctx, tradeAttemptID, StatusSubmitted); err != nil {
		return result, err
	}
	s.attachSignature(ctx, tradeAttemptID, result.Signature) //nolint:errcheck
	return result, nil
}

func (s *Service) Confirm(ctx context.Context, tradeAttemptID, signature string) (BroadcastResult, error) {
	if err := ValidateReportInput(tradeAttemptID, signature); err != nil {
		return BroadcastResult{}, err
	}

	if err := s.update(ctx, tradeAttemptID, StatusConfirming); err != nil {
		return BroadcastResult{}, err
	}

	result, err := s.broadcast.WaitForConfirmation(ctx, signature, s.confirmLvl, s.timeout)
	if err != nil {
		s.fail(ctx, tradeAttemptID, ErrorConfirmationTimeout, err.Error())
		return result, err
	}
	if !result.Confirmed {
		s.fail(ctx, tradeAttemptID, ErrorTransactionFailed, "transaction failed on-chain")
		return result, errors.New("transaction failed on-chain")
	}

	s.markConfirmed(ctx, tradeAttemptID) //nolint:errcheck
	return result, nil
}

func (s *Service) ReportAndVerify(ctx context.Context, tradeAttemptID string) (string, error) {
	attempt, err := s.getAttempt(ctx, tradeAttemptID)
	if err != nil {
		return "", err
	}
	if attempt == nil {
		return "", errors.New("trade attempt not found")
	}
	if attempt.SolanaSignature == "" {
		return "", errors.New("no signature to report")
	}

	if err := s.update(ctx, tradeAttemptID, StatusReporting); err != nil {
		return "", err
	}

	reference, err := s.panta.ReportTransaction(ctx, attempt.MarketID, attempt.Side, attempt.AmountUSDC, attempt.SolanaSignature, attempt.QuoteReference)
	if err != nil {
		s.fail(ctx, tradeAttemptID, ErrorPantaReportFailed, err.Error())
		return "", err
	}
	s.attachPantaRef(ctx, tradeAttemptID, reference) //nolint:errcheck

	verification, err := s.panta.VerifyTransaction(ctx, attempt.SolanaSignature)
	if err != nil {
		s.fail(ctx, tradeAttemptID, ErrorPantaVerificationFailed, err.Error())
		return reference, err
	}
	if verification.Status != "verified" && verification.Status != "pending" {
		return reference, errors.New("Panta verification failed")
	}

	s.markVerified(ctx, tradeAttemptID) //nolint:errcheck
	return reference, nil
}

func (s *Service) RefreshPositions(ctx context.Context, walletPubkey string) ([]PantaPosition, error) {
	positions, err := s.panta.GetPositions(ctx, walletPubkey)
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (s *Service) Complete(ctx context.Context, tradeAttemptID string) error {
	return s.update(ctx, tradeAttemptID, StatusCompleted)
}

func (s *Service) GetAttempt(ctx context.Context, id string) (*Attempt, error) {
	return s.getAttempt(ctx, id)
}

func (s *Service) getAttempt(ctx context.Context, id string) (*Attempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, errors.New("storage unavailable")
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	attempt, err := s.store.GetAttempt(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return attempt, nil
}

func (s *Service) update(ctx context.Context, id string, status Status) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errors.New("storage unavailable")
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.store.UpdateStatus(ctx, tx, id, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) fail(ctx context.Context, id string, code ErrorCode, message string) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_ = s.store.RecordFailure(ctx, tx, id, code, message)
	_ = tx.Commit(ctx)
}

func (s *Service) attachSignature(ctx context.Context, id, signature string) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_ = s.store.AttachSignature(ctx, tx, id, signature)
	_ = tx.Commit(ctx)
}

func (s *Service) attachPantaRef(ctx context.Context, id, reference string) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_ = s.store.AttachPantaReference(ctx, tx, id, reference)
	_ = tx.Commit(ctx)
}

func (s *Service) markConfirmed(ctx context.Context, id string) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_ = s.store.MarkConfirmed(ctx, tx, id)
	_ = tx.Commit(ctx)
}

func (s *Service) markVerified(ctx context.Context, id string) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_ = s.store.MarkVerified(ctx, tx, id)
	_ = tx.Commit(ctx)
}
