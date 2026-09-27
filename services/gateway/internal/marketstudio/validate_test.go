package marketstudio

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func futureDate(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format("2006-01-02")
}

func validDraft() *Draft {
	return &Draft{
		Question:            "ETH will trade above $5,000 on 2027-01-01",
		ResolutionCriteria:  "Resolves YES if the ETH/USD daily close published by the named index is strictly greater than $5,000 at 00:00 UTC on the resolution date.",
		SourcesOfTruth:      []string{"https://www.coingecko.com/en/ethereum"},
		Category:            "crypto",
		ResolutionDate:      futureDate(120),
		ImageURL:            "https://cdn.example.com/eth-1024.webp",
		MarketType:          "standard",
		OutcomeYes:          "ETH closes above $5,000",
		OutcomeNo:           "ETH closes at or below $5,000",
		ResolutionConfirmed: true,
	}
}

func errorCodes(report ValidationReport) map[string]bool {
	codes := make(map[string]bool)
	for _, issue := range report.Errors() {
		codes[issue.Code] = true
	}
	return codes
}

func TestValidDraftPasses(t *testing.T) {
	report := ValidateDraft(validDraft())
	if !report.Valid {
		t.Fatalf("expected valid draft, got %+v", report.Issues)
	}
	if len(report.Errors()) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors())
	}
}

func TestNilDraftIsBlockingAndNotAPanic(t *testing.T) {
	report := ValidateDraft(nil)
	if report.Valid {
		t.Fatal("nil draft must not be valid")
	}
	if !report.NeedsClarification {
		t.Fatal("nil draft must request clarification")
	}
	if !errorCodes(report)["INVALID_DRAFT"] {
		t.Fatalf("expected INVALID_DRAFT, got %+v", report.Issues)
	}
}

func TestUnconfirmedResolutionSourceBlocksCreation(t *testing.T) {
	draft := validDraft()
	draft.ResolutionConfirmed = false
	report := ValidateDraft(draft)
	if report.Valid {
		t.Fatal("an unconfirmed resolution source must block creation")
	}
	if !errorCodes(report)["RESOLUTION_SOURCE_REQUIRES_CONFIRMATION"] {
		t.Fatalf("expected RESOLUTION_SOURCE_REQUIRES_CONFIRMATION, got %+v", report.Issues)
	}
}

func TestMissingResolutionSourceIsReported(t *testing.T) {
	draft := validDraft()
	draft.SourcesOfTruth = nil
	report := ValidateDraft(draft)
	if !errorCodes(report)["RESOLUTION_SOURCE_MISSING"] {
		t.Fatalf("expected RESOLUTION_SOURCE_MISSING, got %+v", report.Issues)
	}
}

func TestVagueResolutionSourceIsRejected(t *testing.T) {
	for _, source := range []string{"TBD", "official source", "to be determined", "some source"} {
		draft := validDraft()
		draft.SourcesOfTruth = []string{source}
		report := ValidateDraft(draft)
		if !errorCodes(report)["RESOLUTION_SOURCE_UNVERIFIED"] {
			t.Errorf("source %q: expected RESOLUTION_SOURCE_UNVERIFIED, got %+v", source, report.Issues)
		}
	}
}

func TestTooManySourcesIsRejected(t *testing.T) {
	draft := validDraft()
	for i := 0; i < MaxSourcesOfTruth+1; i++ {
		draft.SourcesOfTruth = append(draft.SourcesOfTruth, "https://example.com/"+string(rune('a'+i)))
	}
	report := ValidateDraft(draft)
	if !errorCodes(report)["RESOLUTION_SOURCE_LIMIT"] {
		t.Fatalf("expected RESOLUTION_SOURCE_LIMIT, got %+v", report.Issues)
	}
}

func TestQuestionMustBeDeclarative(t *testing.T) {
	draft := validDraft()
	draft.Question = "Will ETH be above $5,000 on 2027-01-01?"
	report := ValidateDraft(draft)
	if !errorCodes(report)["QUESTION_NOT_A_CLAIM"] {
		t.Fatalf("expected QUESTION_NOT_A_CLAIM, got %+v", report.Issues)
	}
}

func TestQuestionLengthLimitMatchesPanta(t *testing.T) {
	draft := validDraft()
	draft.Question = "E" + strings.Repeat("x", MaxQuestionLength+20)
	report := ValidateDraft(draft)
	if !errorCodes(report)["QUESTION_TOO_LONG"] {
		t.Fatalf("expected QUESTION_TOO_LONG, got %+v", report.Issues)
	}
}

func TestUnknownCategoryIsRejected(t *testing.T) {
	draft := validDraft()
	draft.Category = "not-a-real-category"
	report := ValidateDraft(draft)
	if !errorCodes(report)["CATEGORY_INVALID"] {
		t.Fatalf("expected CATEGORY_INVALID, got %+v", report.Issues)
	}
}

func TestKnownCategoriesAreAccepted(t *testing.T) {
	for _, category := range PantaCategories {
		draft := validDraft()
		draft.Category = category
		if report := ValidateDraft(draft); !report.Valid {
			t.Errorf("category %q should be accepted, got %+v", category, report.Errors())
		}
	}
}

func TestMissingImageURLIsReported(t *testing.T) {
	draft := validDraft()
	draft.ImageURL = ""
	report := ValidateDraft(draft)
	if !errorCodes(report)["IMAGE_URL_MISSING"] {
		t.Fatalf("expected IMAGE_URL_MISSING, got %+v", report.Issues)
	}
}

func TestInvalidImageURLsAreRejected(t *testing.T) {
	cases := []struct {
		code string
		url  string
	}{
		{"IMAGE_URL_INVALID", "/local/eth.png"},
		{"IMAGE_URL_DATA_UNSUPPORTED", "data:image/png;base64,iVBORw0KGgo="},
		{"IMAGE_URL_PRIVATE_HOST", "http://localhost:3000/eth.png"},
		{"IMAGE_URL_PRIVATE_HOST", "http://192.168.1.10/eth.png"},
		{"IMAGE_URL_INVALID", "https://"},
	}
	for _, testCase := range cases {
		draft := validDraft()
		draft.ImageURL = testCase.url
		report := ValidateDraft(draft)
		if !errorCodes(report)[testCase.code] {
			t.Errorf("url %q: expected %s, got %+v", testCase.url, testCase.code, report.Issues)
		}
	}
}

func TestExpiredResolutionDateIsRejected(t *testing.T) {
	draft := validDraft()
	draft.ResolutionDate = futureDate(-5)
	report := ValidateDraft(draft)
	if !errorCodes(report)["RESOLUTION_DATE_EXPIRED"] {
		t.Fatalf("expected RESOLUTION_DATE_EXPIRED, got %+v", report.Issues)
	}
}

func TestTodayResolutionDateIsTooSoon(t *testing.T) {
	draft := validDraft()
	today := time.Now().UTC().Format("2006-01-02")
	draft.ResolutionDate = today
	report := ValidateDraft(draft)
	if !errorCodes(report)["RESOLUTION_DATE_EXPIRED"] {
		t.Fatalf("expected RESOLUTION_DATE_EXPIRED for today, got %+v", report.Issues)
	}
}

func TestInvalidDateStringIsRejected(t *testing.T) {
	draft := validDraft()
	draft.ResolutionDate = "not-a-date"
	report := ValidateDraft(draft)
	if !errorCodes(report)["RESOLUTION_DATE_INVALID"] {
		t.Fatalf("expected RESOLUTION_DATE_INVALID, got %+v", report.Issues)
	}
}

func TestInvertedDatesAreRejected(t *testing.T) {
	draft := validDraft()
	draft.StartDate = futureDate(90)
	draft.EndDate = futureDate(30)
	draft.ResolutionDate = futureDate(60)
	report := ValidateDraft(draft)
	if !errorCodes(report)["DATE_ORDER_INVALID"] {
		t.Fatalf("expected DATE_ORDER_INVALID, got %+v", report.Issues)
	}
}

func TestMissingOutcomesAreReported(t *testing.T) {
	draft := validDraft()
	draft.OutcomeYes = ""
	draft.OutcomeNo = ""
	report := ValidateDraft(draft)
	codes := errorCodes(report)
	if !codes["OUTCOME_YES_MISSING"] || !codes["OUTCOME_NO_MISSING"] {
		t.Fatalf("expected both outcome errors, got %+v", report.Issues)
	}
}

// ---- Draft fingerprint: the integrity anchor --------------------------- //

func TestFingerprintIsStableForIdenticalDrafts(t *testing.T) {
	if DraftFingerprint(validDraft()) != DraftFingerprint(validDraft()) {
		t.Fatal("identical drafts must produce identical fingerprints")
	}
}

func TestFingerprintChangesOnEveryMaterialField(t *testing.T) {
	baseline := DraftFingerprint(validDraft())

	mutations := map[string]func(*Draft){
		"question":            func(d *Draft) { d.Question = "ETH will trade above $6,000 on 2027-01-01" },
		"resolution_criteria": func(d *Draft) { d.ResolutionCriteria = "A completely different settlement rule." },
		"category":            func(d *Draft) { d.Category = "finance" },
		"resolution_date":     func(d *Draft) { d.ResolutionDate = futureDate(200) },
		"image_url":           func(d *Draft) { d.ImageURL = "https://cdn.example.com/other.webp" },
		"market_type":         func(d *Draft) { d.MarketType = "breaking" },
		"title":               func(d *Draft) { d.Title = "A new title" },
		"sources":             func(d *Draft) { d.SourcesOfTruth = []string{"https://example.com/alt"} },
		"region":              func(d *Draft) { d.Region = "US" },
	}
	for name, mutate := range mutations {
		draft := validDraft()
		mutate(draft)
		if got := DraftFingerprint(draft); got == baseline {
			t.Errorf("changing %s must change the fingerprint", name)
		}
	}
}

func TestFingerprintIgnoresPresentationOnlyFields(t *testing.T) {
	baseline := DraftFingerprint(validDraft())

	// A private note or outcome wording change must NOT invalidate a quote the
	// user already approved; those fields are never sent to Panta.
	draft := validDraft()
	draft.Notes = "just thinking out loud"
	draft.OutcomeYes = "ETH settles above the threshold"

	if got := DraftFingerprint(draft); got != baseline {
		t.Fatal("presentation-only fields must not change the fingerprint")
	}
}

func TestFingerprintIsSourceOrderInsensitive(t *testing.T) {
	first := validDraft()
	first.SourcesOfTruth = []string{"https://a.example.com", "https://b.example.com"}
	second := validDraft()
	second.SourcesOfTruth = []string{"https://b.example.com", "https://a.example.com"}

	if DraftFingerprint(first) != DraftFingerprint(second) {
		t.Fatal("source ordering must not change the fingerprint")
	}
}

// ---- Panta parameter construction --------------------------------------- //

func TestPantaParamsUseOnlyVerifiedFields(t *testing.T) {
	params, err := BuildPantaCreateParams(validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.Wallet != "7xWallet" {
		t.Errorf("expected wallet, got %s", params.Wallet)
	}
	if params.ResolutionRule == "" {
		t.Error("resolutionRule must be populated")
	}
	if len(params.SourcesOfTruth) != 1 {
		t.Errorf("expected 1 source, got %d", len(params.SourcesOfTruth))
	}
	if params.MarketType != "standard" {
		t.Errorf("expected standard market, got %s", params.MarketType)
	}
}

func TestPantaParamsDefaultMarketType(t *testing.T) {
	draft := validDraft()
	draft.MarketType = ""
	params, err := BuildPantaCreateParams(draft, "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.MarketType != "standard" {
		t.Errorf("expected default standard, got %s", params.MarketType)
	}
}

func TestPantaParamsHonourTimeOrdering(t *testing.T) {
	params, err := BuildPantaCreateParams(validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(params.StartTime < params.EndTime && params.EndTime <= params.ResolutionTime) {
		t.Fatalf("expected start < end <= resolution, got %d/%d/%d",
			params.StartTime, params.EndTime, params.ResolutionTime)
	}
}

func TestPantaParamsWidenWindowWhenOnlyResolutionDateGiven(t *testing.T) {
	draft := validDraft()
	draft.StartDate = ""
	draft.EndDate = ""
	params, err := BuildPantaCreateParams(draft, "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(params.StartTime < params.EndTime && params.EndTime <= params.ResolutionTime) {
		t.Fatalf("window widening failed: %d/%d/%d",
			params.StartTime, params.EndTime, params.ResolutionTime)
	}
}

func TestPantaParamsRejectUnparseableDate(t *testing.T) {
	draft := validDraft()
	draft.ResolutionDate = "31/12/2027"
	if _, err := BuildPantaCreateParams(draft, "7xWallet"); err == nil {
		t.Fatal("expected an error for an unparseable date rather than a guess")
	}
}

func TestPantaParamsRequireWallet(t *testing.T) {
	if _, err := BuildPantaCreateParams(validDraft(), "  "); !errors.Is(err, ErrWalletRequired) {
		t.Fatalf("expected ErrWalletRequired, got %v", err)
	}
}

// ---- Request validation ------------------------------------------------- //

func TestQuoteRequestRejectsUnconfirmedSource(t *testing.T) {
	draft := validDraft()
	draft.ResolutionConfirmed = false
	err := ValidateQuoteRequest(draft, "7xWallet")
	if !errors.Is(err, ErrDraftRequired) && !errors.Is(err, ErrResolutionSourceNeeded) {
		t.Fatalf("expected the unconfirmed source to be rejected, got %v", err)
	}
}

func TestQuoteRequestRejectsInvalidDraft(t *testing.T) {
	draft := validDraft()
	draft.ImageURL = ""
	if err := ValidateQuoteRequest(draft, "7xWallet"); err == nil {
		t.Fatal("a draft missing a required Panta field must not be quotable")
	}
}

func TestQuoteRequestRequiresWallet(t *testing.T) {
	if err := ValidateQuoteRequest(validDraft(), ""); !errors.Is(err, ErrWalletRequired) {
		t.Fatalf("expected ErrWalletRequired, got %v", err)
	}
}

func TestBuildRequestRejectsWrongWallet(t *testing.T) {
	attempt := &Attempt{ID: "a1", WalletAddress: "7xWallet", CreateID: "c1"}
	if err := ValidateBuildRequest(attempt, "9xOther"); err == nil {
		t.Fatal("a build must not proceed under a different wallet than was quoted")
	}
}

func TestBuildRequestRequiresQuote(t *testing.T) {
	attempt := &Attempt{ID: "a1", WalletAddress: "7xWallet"}
	if err := ValidateBuildRequest(attempt, "7xWallet"); !errors.Is(err, ErrQuoteNotFound) {
		t.Fatalf("expected ErrQuoteNotFound, got %v", err)
	}
}

func TestValidateBroadcastRejectsDraftChangedAfterQuote(t *testing.T) {
	attempt := &Attempt{ID: "a1", WalletAddress: "7xWallet", CreateID: "c1", DraftHash: "hash-a"}
	err := ValidateBroadcastRequest(attempt, "7xWallet", "c2ln", "hash-b")
	if !errors.Is(err, ErrDraftChanged) {
		t.Fatalf("expected ErrDraftChanged, got %v", err)
	}
}

func TestBroadcastAcceptsUnchangedDraft(t *testing.T) {
	attempt := &Attempt{ID: "a1", WalletAddress: "7xWallet", CreateID: "c1", DraftHash: "hash-a"}
	if err := ValidateBroadcastRequest(attempt, "7xWallet", "c2ln", "hash-a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterRejectsMismatchedSignature(t *testing.T) {
	attempt := &Attempt{ID: "a1", CreateID: "c1", SolanaSignature: "sigA"}
	if err := ValidateRegisterRequest(attempt, "sigB"); err == nil {
		t.Fatal("registering a different signature than was broadcast must be rejected")
	}
}

func TestRegisterRequiresSignature(t *testing.T) {
	attempt := &Attempt{ID: "a1", CreateID: "c1"}
	if err := ValidateRegisterRequest(attempt, ""); !errors.Is(err, ErrSignatureRequired) {
		t.Fatalf("expected ErrSignatureRequired, got %v", err)
	}
}

// ---- Base units: the no-float64 rule ----------------------------------- //

func TestParseBaseUnitsAcceptsIntegerStrings(t *testing.T) {
	for _, value := range []string{"0", "1", "50000000", "999999999999"} {
		if _, err := ParseBaseUnits(value); err != nil {
			t.Errorf("expected %q to parse, got %v", value, err)
		}
	}
}

func TestParseBaseUnitsRejectsNonIntegers(t *testing.T) {
	// Each of these would silently corrupt a USDC amount if a float were used.
	for _, value := range []string{"10.50", "1e6", "-500", "+500", "0x10", "abc", "", " 5 5"} {
		if _, err := ParseBaseUnits(value); err == nil {
			t.Errorf("expected %q to be rejected as a base-unit string", value)
		}
	}
}

func TestBaseUnitsOverflowIsRejected(t *testing.T) {
	if _, err := ParseBaseUnits("99999999999999999999999"); err == nil {
		t.Fatal("expected an out-of-range amount to be rejected")
	}
}

// ---- Clarification messaging ------------------------------------------- //

func TestSummarizeMissingUsesRequiredWording(t *testing.T) {
	message, fields := SummarizeMissing([]string{"resolution_source", "deadline"})
	if !strings.Contains(message, "Your question needs more detail before a market can be created.") {
		t.Errorf("missing required wording, got %q", message)
	}
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestSummarizeMissingIsEmptyWhenNothingMissing(t *testing.T) {
	message, fields := SummarizeMissing(nil)
	if message != "" || fields != nil {
		t.Errorf("expected empty result, got %q / %v", message, fields)
	}
}

func TestCollectMissingFieldsNamesGaps(t *testing.T) {
	draft := validDraft()
	draft.SourcesOfTruth = nil
	draft.ResolutionConfirmed = false
	draft.ImageURL = ""

	missing := CollectMissingFields(draft)
	wanted := map[string]bool{
		"resolution_source":              true,
		"resolution_source_confirmation": true,
		"image_url":                      true,
	}
	for _, field := range missing {
		delete(wanted, field)
	}
	if len(wanted) != 0 {
		t.Fatalf("missing fields not reported: %v (got %v)", wanted, missing)
	}
}
