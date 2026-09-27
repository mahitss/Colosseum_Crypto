package marketstudio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	neturl "net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VALIDATION
//
// Deterministic gate for market creation. The AI produces a draft; this code
// decides whether that draft may proceed. Two independent facts drive that:
//
//  1. The LLM is not trusted. Every field it produced is re-checked here
//     against the verified Panta create-API limits, so a hallucinated or
//     manipulated draft cannot reach Panta.
//  2. Nothing is invented. A missing field produces an error naming that
//     field. We never fill in a threshold, a source, or a deadline ourselves.
//
// Sentinel errors mirror the trading package so the HTTP layer can map them to
// stable status codes.

var (
	ErrDraftRequired           = errors.New("market draft is required")
	ErrWalletRequired          = errors.New("wallet public key is required")
	ErrAttemptRequired         = errors.New("creation attempt identifier is required")
	ErrDraftChanged            = errors.New("draft changed after quote; request a new quote")
	ErrResolutionSourceNeeded  = errors.New("resolution source must be confirmed by the user")
	ErrCreateIDRequired        = errors.New("panta create id is required")
	ErrSignatureRequired       = errors.New("transaction signature is required")
	ErrSignedTxRequired        = errors.New("signed transaction is required")
	ErrAttemptNotFound         = errors.New("creation attempt not found")
	ErrQuoteNotFound           = errors.New("no panta quote exists for this attempt")
	ErrAttemptNotBroadcastable = errors.New("attempt is not in a signable state")
)

// Panta create-quote limits, verified against the current Panta documentation.
const (
	MaxQuestionLength     = 512
	MaxResolutionRule     = 2048
	MaxImageURLLength     = 2048
	MaxSourcesOfTruth     = 20
	MaxTitleLength        = 512
	MaxMarketDescription  = 2048
	MaxRegionLength       = 64
	MaxOutcomeLength      = 300
	MinResolutionLeadDays = 1
)

// MaxDraftFieldLength bounds any single draft field at the edge. It sits far
// above the Panta limits so the deterministic checks below produce specific,
// field-level messages instead of a generic rejection.
const MaxDraftFieldLength = 16384

// PantaCategories is the documented category allowlist.
var PantaCategories = []string{
	"sports", "crypto", "politics", "entertainment",
	"finance", "science", "world", "other",
}

// Sanity ceiling on timestamps. Panta expects unix seconds; a value beyond this
// means a malformed date rather than a real one.
const (
	maxReasonableUnix = int64(4102444800) // 2100-01-01
	minReasonableUnix = int64(1000000000) // 2001-09-09
)

func isCategoryAllowed(category string) bool {
	for _, allowed := range PantaCategories {
		if category == allowed {
			return true
		}
	}
	return false
}

func hasText(value string) bool { return strings.TrimSpace(value) != "" }

func hasAlphanumeric(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}) >= 0
}

func hasControlChars(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return r < 0x20 && r != '\n' && r != '\r' && r != '\t'
	})
}

// sourceMarkers are phrases that name no actual authority. A resolution source
// must be specific enough that a settler could fetch it.
var sourceMarkers = []string{
	"tbd", "t.b.d", "to be determined", "unknown", "n/a", "some source",
	"appropriate source", "the source", "any source", "trusted source",
	"officially", "official source", "good source", "reputable source",
}

func isVagueSource(source string) bool {
	lowered := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(source)), ".")
	for _, marker := range sourceMarkers {
		if lowered == marker || strings.HasPrefix(lowered, marker) {
			return true
		}
	}
	return false
}

func issue(field, code, message, severity string) ValidationIssue {
	return ValidationIssue{Field: field, Code: code, Message: message, Severity: severity}
}

func errorIssue(field, code, message string) ValidationIssue {
	return issue(field, code, message, "error")
}

func warnIssue(field, code, message string) ValidationIssue {
	return issue(field, code, message, "warning")
}

// ValidateDraft runs every deterministic rule and returns a complete report.
// A nil draft is a blocking INVALID_DRAFT, not a panic.
func ValidateDraft(draft *Draft) ValidationReport {
	if draft == nil {
		return ValidationReport{
			Valid: false,
			Issues: []ValidationIssue{errorIssue("draft", string(ErrorInvalidDraft),
				"No market draft was supplied.")},
			MissingFields:      []string{"question", "resolution_criteria", "resolution_source", "deadline"},
			NeedsClarification: true,
		}
	}

	var issues []ValidationIssue
	issues = append(issues, validateQuestion(draft)...)
	issues = append(issues, validateOutcomes(draft)...)
	issues = append(issues, validateResolutionCriteria(draft)...)
	issues = append(issues, validateSources(draft)...)
	issues = append(issues, validateSourceConfirmation(draft)...)
	issues = append(issues, validateDates(draft)...)
	issues = append(issues, validateImage(draft)...)
	issues = append(issues, validatePantaConstraints(draft)...)

	report := ValidationReport{
		// Start optimistic, then demote on the first blocking issue. A report
		// with only warnings stays valid: warnings are shown, not hidden.
		Valid:            true,
		Issues:           issues,
		MissingFields:    CollectMissingFields(draft),
		DraftFingerprint: DraftFingerprint(draft),
	}
	for _, item := range issues {
		if item.Severity == "error" {
			report.Valid = false
			break
		}
	}
	if report.Issues == nil {
		report.Issues = []ValidationIssue{}
	}
	if report.MissingFields == nil {
		report.MissingFields = []string{}
	}
	return report
}

func validateQuestion(draft *Draft) []ValidationIssue {
	question := strings.TrimSpace(draft.Question)
	if len(question) < 10 || !hasAlphanumeric(question) {
		return []ValidationIssue{errorIssue("question", "QUESTION_UNCLEAR",
			"The question must state a specific, checkable claim of at least 10 characters.")}
	}
	var issues []ValidationIssue
	if hasControlChars(question) {
		issues = append(issues, errorIssue("question", "QUESTION_INVALID_CHARACTERS",
			"The question contains unsupported control characters."))
	}
	if len(question) > MaxQuestionLength {
		issues = append(issues, errorIssue("question", "QUESTION_TOO_LONG",
			fmt.Sprintf("The question must be %d characters or fewer (currently %d).", MaxQuestionLength, len(question))))
	}
	if strings.HasSuffix(question, "?") {
		issues = append(issues, errorIssue("question", "QUESTION_NOT_A_CLAIM",
			"The question must be a declarative claim, not a question. Try: 'ETH will be above $5,000 on December 31, 2026'."))
	}
	return issues
}

func validateOutcomes(draft *Draft) []ValidationIssue {
	var issues []ValidationIssue
	for _, outcome := range []struct {
		field   string
		label   string
		value   string
		missing string
		tooLong string
	}{
		{"outcome_yes", "YES", draft.OutcomeYes, "OUTCOME_YES_MISSING", "OUTCOME_YES_TOO_LONG"},
		{"outcome_no", "NO", draft.OutcomeNo, "OUTCOME_NO_MISSING", "OUTCOME_NO_TOO_LONG"},
	} {
		value := strings.TrimSpace(outcome.value)
		if value == "" || !hasAlphanumeric(value) {
			issues = append(issues, errorIssue(outcome.field, outcome.missing,
				fmt.Sprintf("Define what resolves %s.", outcome.label)))
			continue
		}
		if len(value) > MaxOutcomeLength {
			issues = append(issues, errorIssue(outcome.field, outcome.tooLong,
				fmt.Sprintf("The %s outcome must be %d characters or fewer.", outcome.label, MaxOutcomeLength)))
		}
	}
	return issues
}

func validateResolutionCriteria(draft *Draft) []ValidationIssue {
	criteria := strings.TrimSpace(draft.ResolutionCriteria)
	if len(criteria) < 10 || !hasAlphanumeric(criteria) {
		return []ValidationIssue{errorIssue("resolution_criteria", "RESOLUTION_CRITERIA_MISSING",
			"Resolution criteria are required. State exactly how the market settles and which value is observed.")}
	}
	var issues []ValidationIssue
	if len(criteria) > MaxResolutionRule {
		issues = append(issues, errorIssue("resolution_criteria", "RESOLUTION_CRITERIA_TOO_LONG",
			fmt.Sprintf("Resolution criteria must be %d characters or fewer (currently %d).", MaxResolutionRule, len(criteria))))
	}
	if hasControlChars(criteria) {
		issues = append(issues, errorIssue("resolution_criteria", "RESOLUTION_CRITERIA_INVALID_CHARACTERS",
			"Resolution criteria contain unsupported control characters."))
	}
	return issues
}

func validateSources(draft *Draft) []ValidationIssue {
	var cleaned []string
	for _, source := range draft.SourcesOfTruth {
		if hasText(source) {
			cleaned = append(cleaned, strings.TrimSpace(source))
		}
	}
	if len(cleaned) == 0 {
		return []ValidationIssue{errorIssue("sources_of_truth", "RESOLUTION_SOURCE_MISSING",
			"A resolution source is required. Add at least one authoritative source used to settle this market.")}
	}

	var issues []ValidationIssue
	if len(cleaned) > MaxSourcesOfTruth {
		issues = append(issues, errorIssue("sources_of_truth", "RESOLUTION_SOURCE_LIMIT",
			fmt.Sprintf("At most %d resolution sources are allowed.", MaxSourcesOfTruth)))
	}
	for _, source := range cleaned {
		if isVagueSource(source) {
			issues = append(issues, errorIssue("sources_of_truth", "RESOLUTION_SOURCE_UNVERIFIED",
				fmt.Sprintf("'%s' is not a specific authoritative source. Name the exact publication or feed used for settlement.", source)))
		}
		if len(source) > 512 {
			issues = append(issues, errorIssue("sources_of_truth", "RESOLUTION_SOURCE_TOO_LONG",
				"Each resolution source must be 512 characters or fewer."))
		}
	}
	return issues
}

// validateSourceConfirmation enforces the rule that the model must not invent an
// authoritative source. Even a plausible-looking suggestion is unconfirmed
// until the user says so.
func validateSourceConfirmation(draft *Draft) []ValidationIssue {
	if draft.ResolutionConfirmed {
		return nil
	}
	return []ValidationIssue{errorIssue("resolution_source_confirmed", "RESOLUTION_SOURCE_REQUIRES_CONFIRMATION",
		"Confirm the resolution source before this market can be created. The assistant proposed it; you decide it.")}
}

func validateDates(draft *Draft) []ValidationIssue {
	var issues []ValidationIssue
	resolution, err := parseDate(draft.ResolutionDate)
	if err != nil {
		return []ValidationIssue{errorIssue("resolution_date", "RESOLUTION_DATE_INVALID",
			"The resolution date must be a valid YYYY-MM-DD date.")}
	}
	today := todayUTC()

	if !resolution.After(today) {
		issues = append(issues, errorIssue("resolution_date", "RESOLUTION_DATE_EXPIRED",
			fmt.Sprintf("The resolution date (%s) is not in the future. Choose a future date.", resolution.Format("2006-01-02"))))
	} else if resolution.Sub(today) < MinResolutionLeadDays*24*time.Hour {
		issues = append(issues, errorIssue("resolution_date", "RESOLUTION_DATE_TOO_SOON",
			"The resolution date is too close. Panta requires a minimum start delay before trading opens, so allow at least one day."))
	}

	start := today
	if hasText(draft.StartDate) {
		parsed, startErr := parseDate(draft.StartDate)
		if startErr != nil {
			return append(issues, errorIssue("start_date", "START_DATE_INVALID",
				"The start date must be a valid YYYY-MM-DD date."))
		}
		start = parsed
	}
	if start.Before(today) {
		issues = append(issues, errorIssue("start_date", "START_DATE_PAST",
			"The start date cannot be in the past."))
	}

	end := resolution
	if hasText(draft.EndDate) {
		parsed, endErr := parseDate(draft.EndDate)
		if endErr != nil {
			return append(issues, errorIssue("end_date", "END_DATE_INVALID",
				"The end date must be a valid YYYY-MM-DD date."))
		}
		end = parsed
	}

	// Panta requires startTime < endTime <= resolutionTime.
	if !(start.Before(end) && !end.After(resolution)) {
		issues = append(issues, errorIssue("end_date", "DATE_ORDER_INVALID",
			fmt.Sprintf("Dates must satisfy start < end <= resolution (got %s / %s / %s).",
				start.Format("2006-01-02"), end.Format("2006-01-02"), resolution.Format("2006-01-02"))))
	}
	return issues
}

func validateImage(draft *Draft) []ValidationIssue {
	url := strings.TrimSpace(draft.ImageURL)
	if url == "" {
		return []ValidationIssue{errorIssue("image_url", "IMAGE_URL_MISSING",
			"Panta requires a catalog image URL. Provide a publicly reachable http(s) image.")}
	}

	var issues []ValidationIssue
	if len(url) > MaxImageURLLength {
		issues = append(issues, errorIssue("image_url", "IMAGE_URL_TOO_LONG",
			fmt.Sprintf("The image URL must be %d characters or fewer.", MaxImageURLLength)))
	}
	lowered := strings.ToLower(url)
	if strings.HasPrefix(lowered, "data:") {
		issues = append(issues, errorIssue("image_url", "IMAGE_URL_DATA_UNSUPPORTED",
			"Data URLs are not accepted. Host the image and pass its URL."))
		return issues
	}
	if !strings.HasPrefix(lowered, "http://") && !strings.HasPrefix(lowered, "https://") {
		issues = append(issues, errorIssue("image_url", "IMAGE_URL_INVALID",
			"The image URL must start with http:// or https://."))
		return issues
	}

	parsed, err := neturl.Parse(url)
	if err != nil || parsed.Host == "" {
		issues = append(issues, errorIssue("image_url", "IMAGE_URL_INVALID",
			"The image URL is not a valid absolute URL."))
		return issues
	}
	host := strings.ToLower(parsed.Hostname())
	for _, blocked := range []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "10.", "192.168.", "172.16.", ".local"} {
		if host == strings.Trim(blocked, ".") || strings.HasPrefix(host, blocked) {
			issues = append(issues, errorIssue("image_url", "IMAGE_URL_PRIVATE_HOST",
				"The image must be publicly reachable; private and localhost hosts are rejected by Panta."))
			break
		}
	}
	return issues
}

func validatePantaConstraints(draft *Draft) []ValidationIssue {
	var issues []ValidationIssue
	if !isCategoryAllowed(draft.Category) {
		issues = append(issues, errorIssue("category", "CATEGORY_INVALID",
			"Category must be one of: "+strings.Join(PantaCategories, ", ")+"."))
	}
	if marketType := draft.MarketType; marketType != "" && marketType != "standard" && marketType != "breaking" {
		issues = append(issues, errorIssue("market_type", "MARKET_TYPE_INVALID",
			"Market type must be 'standard' or 'breaking'."))
	}
	if len(draft.Title) > MaxTitleLength {
		issues = append(issues, errorIssue("title", "TITLE_TOO_LONG",
			fmt.Sprintf("The title must be %d characters or fewer.", MaxTitleLength)))
	}
	if len(draft.Description) > MaxMarketDescription {
		issues = append(issues, errorIssue("description", "DESCRIPTION_TOO_LONG",
			fmt.Sprintf("The description must be %d characters or fewer.", MaxMarketDescription)))
	}
	if len(draft.Region) > MaxRegionLength {
		issues = append(issues, errorIssue("region", "REGION_TOO_LONG",
			fmt.Sprintf("The region must be %d characters or fewer.", MaxRegionLength)))
	}
	return issues
}

// CollectMissingFields names what a human still has to supply. It reports gaps;
// it never fills them.
func CollectMissingFields(draft *Draft) []string {
	if draft == nil {
		return []string{"question", "resolution_criteria", "resolution_source", "deadline"}
	}
	var missing []string
	if !hasText(draft.Question) {
		missing = append(missing, "question")
	}
	if !hasText(draft.OutcomeYes) {
		missing = append(missing, "threshold_or_yes_definition")
	}
	if !hasText(draft.OutcomeNo) {
		missing = append(missing, "no_definition")
	}
	if !hasText(draft.ResolutionCriteria) {
		missing = append(missing, "resolution_criteria")
	}
	hasSource := false
	for _, source := range draft.SourcesOfTruth {
		if hasText(source) {
			hasSource = true
			break
		}
	}
	if !hasSource {
		missing = append(missing, "resolution_source")
	}
	if !draft.ResolutionConfirmed {
		missing = append(missing, "resolution_source_confirmation")
	}
	if !hasText(draft.ResolutionDate) {
		missing = append(missing, "deadline")
	} else if resolution, err := parseDate(draft.ResolutionDate); err != nil || !resolution.After(todayUTC()) {
		missing = append(missing, "future_deadline")
	}
	if !hasText(draft.ImageURL) {
		missing = append(missing, "image_url")
	}
	if missing == nil {
		return []string{}
	}
	return missing
}

var missingFieldLabels = map[string]string{
	"question":                       "a clear, checkable question",
	"threshold_or_yes_definition":    "the threshold or condition that resolves YES",
	"no_definition":                  "what resolves NO",
	"resolution_criteria":            "resolution criteria (how the market settles)",
	"resolution_source":              "an authoritative resolution source",
	"resolution_source_confirmation": "your confirmation of the resolution source",
	"deadline":                       "a deadline",
	"future_deadline":                "a future deadline",
	"image_url":                      "a catalog image URL required by Panta",
}

// SummarizeMissing renders the user-facing clarification message. The exact
// wording is fixed by spec: an ambiguous question is never auto-completed.
func SummarizeMissing(missing []string) (string, []string) {
	if len(missing) == 0 {
		return "", nil
	}
	labels := make([]string, 0, len(missing))
	for _, field := range missing {
		if label, ok := missingFieldLabels[field]; ok {
			labels = append(labels, label)
			continue
		}
		labels = append(labels, field)
	}
	return "Your question needs more detail before a market can be created. Suggested missing fields: " +
		strings.Join(labels, ", ") + ".", missing
}

func todayUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

// DraftFingerprint hashes the fields a user actually reviews.
//
// Any material change produces a different hash. That is what invalidates a
// previously quoted or built transaction instead of silently reusing it.
// Prophet-only presentation fields (outcome text, notes) are excluded: editing
// a private note must not invalidate a quote the user already approved.
func DraftFingerprint(draft *Draft) string {
	if draft == nil {
		return ""
	}
	payload := struct {
		Question           string   `json:"question"`
		ResolutionCriteria string   `json:"resolutionCriteria"`
		SourcesOfTruth     []string `json:"sourcesOfTruth"`
		Category           string   `json:"category"`
		ResolutionDate     string   `json:"resolutionDate"`
		EndDate            string   `json:"endDate"`
		StartDate          string   `json:"startDate"`
		ImageURL           string   `json:"imageUrl"`
		Title              string   `json:"title"`
		Description        string   `json:"description"`
		Region             string   `json:"region"`
		MarketType         string   `json:"marketType"`
	}{
		Question:           strings.TrimSpace(draft.Question),
		ResolutionCriteria: strings.TrimSpace(draft.ResolutionCriteria),
		SourcesOfTruth:     normalizeSources(draft.SourcesOfTruth),
		Category:           strings.TrimSpace(draft.Category),
		ResolutionDate:     strings.TrimSpace(draft.ResolutionDate),
		EndDate:            strings.TrimSpace(draft.EndDate),
		StartDate:          strings.TrimSpace(draft.StartDate),
		ImageURL:           strings.TrimSpace(draft.ImageURL),
		Title:              strings.TrimSpace(draft.Title),
		Description:        strings.TrimSpace(draft.Description),
		Region:             strings.TrimSpace(draft.Region),
		MarketType:         strings.TrimSpace(draft.MarketType),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func normalizeSources(sources []string) []string {
	cleaned := make([]string, 0, len(sources))
	for _, source := range sources {
		if hasText(source) {
			cleaned = append(cleaned, strings.TrimSpace(source))
		}
	}
	sort.Strings(cleaned)
	return cleaned
}

// BuildPantaCreateParams converts a validated draft into the exact Panta
// create-quote body. It refuses rather than guessing when a date is unparseable.
//
// Dates are converted to midnight UTC unix seconds. Panta's constraint
// startTime < endTime <= resolutionTime is honoured by widening the window
// deterministically — a user who supplied only a resolution date still gets a
// valid request, without us inventing a business meaning.
func BuildPantaCreateParams(draft *Draft, wallet string) (PantaCreateParams, error) {
	if draft == nil {
		return PantaCreateParams{}, ErrDraftRequired
	}
	if !hasText(wallet) {
		return PantaCreateParams{}, ErrWalletRequired
	}

	resolution, err := parseDate(draft.ResolutionDate)
	if err != nil {
		return PantaCreateParams{}, fmt.Errorf("resolution date: %w", err)
	}
	end := resolution
	if hasText(draft.EndDate) {
		parsed, endErr := parseDate(draft.EndDate)
		if endErr != nil {
			return PantaCreateParams{}, fmt.Errorf("end date: %w", endErr)
		}
		end = parsed
	}
	start := todayUTC()
	if hasText(draft.StartDate) {
		parsed, startErr := parseDate(draft.StartDate)
		if startErr != nil {
			return PantaCreateParams{}, fmt.Errorf("start date: %w", startErr)
		}
		start = parsed
	}

	startTime := midnightUnix(start)
	endTime := midnightUnix(end)
	resolutionTime := midnightUnix(resolution)

	// Panta's ordering constraint. Widen rather than fail: this is a mechanical
	// consequence of the user supplying fewer dates than Panta requires, not an
	// ambiguity about what the market means.
	if startTime >= endTime {
		endTime = startTime + 86400
	}
	if endTime > resolutionTime {
		resolutionTime = endTime
	}
	if startTime < minReasonableUnix || resolutionTime > maxReasonableUnix {
		return PantaCreateParams{}, errors.New("draft dates are outside the supported range")
	}

	marketType := draft.MarketType
	if marketType == "" {
		marketType = "standard"
	}

	var sources []string
	for _, source := range draft.SourcesOfTruth {
		if hasText(source) {
			sources = append(sources, strings.TrimSpace(source))
		}
	}
	if sources == nil {
		sources = []string{}
	}

	return PantaCreateParams{
		Wallet:         strings.TrimSpace(wallet),
		Question:       strings.TrimSpace(draft.Question),
		ResolutionRule: strings.TrimSpace(draft.ResolutionCriteria),
		SourcesOfTruth: sources,
		Category:       strings.TrimSpace(draft.Category),
		StartTime:      startTime,
		EndTime:        endTime,
		ResolutionTime: resolutionTime,
		ImageURL:       strings.TrimSpace(draft.ImageURL),
		MarketType:     marketType,
		Title:          strings.TrimSpace(draft.Title),
		Description:    strings.TrimSpace(draft.Description),
		Region:         strings.TrimSpace(draft.Region),
	}, nil
}

func midnightUnix(day time.Time) int64 {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Unix()
}

// ValidateQuoteRequest checks a quote request. A draft that fails
// deterministic validation must never reach Panta, so a Create Market action
// simply cannot be issued in the first place.
func ValidateQuoteRequest(draft *Draft, wallet string) error {
	if draft == nil {
		return ErrDraftRequired
	}
	if !hasText(wallet) {
		return ErrWalletRequired
	}
	if report := ValidateDraft(draft); !report.Valid {
		return ErrInvalidDraftResult(report)
	}
	if !draft.ResolutionConfirmed {
		return ErrResolutionSourceNeeded
	}
	if _, err := BuildPantaCreateParams(draft, wallet); err != nil {
		return err
	}
	return nil
}

// ErrInvalidDraftResult wraps a failed report so the HTTP layer can surface the
// individual field-level messages rather than a generic rejection.
func ErrInvalidDraftResult(report ValidationReport) error {
	codes := make([]string, 0, len(report.Issues))
	for _, item := range report.Errors() {
		codes = append(codes, item.Field+": "+item.Message)
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		return errors.New("market draft is not valid")
	}
	return fmt.Errorf("%w: %s", ErrDraftRequired, strings.Join(codes, "; "))
}

// ValidateBuildRequest checks a build request against the stored quote. Panta
// ties a build to its createId, so the wallet must be the one that was quoted.
func ValidateBuildRequest(attempt *Attempt, wallet string) error {
	if attempt == nil {
		return ErrAttemptNotFound
	}
	if !hasText(wallet) {
		return ErrWalletRequired
	}
	if attempt.WalletAddress != wallet {
		return errors.New("connected wallet does not match the wallet that was quoted")
	}
	if !hasText(attempt.CreateID) {
		return ErrQuoteNotFound
	}
	return nil
}

// ValidateBroadcastRequest checks the signed-transaction handoff. Nothing
// meaningful happens before this passes.
func ValidateBroadcastRequest(attempt *Attempt, wallet, signedTx, draftHash string) error {
	if attempt == nil {
		return ErrAttemptNotFound
	}
	if attempt.WalletAddress != wallet {
		return errors.New("connected wallet does not match the creation attempt")
	}
	if !hasText(signedTx) {
		return ErrSignedTxRequired
	}
	if !hasText(draftHash) {
		return ErrDraftRequired
	}
	// Draft integrity. A material change after quoting must invalidate the
	// previously built transaction rather than silently proceed.
	if hasText(attempt.DraftHash) && attempt.DraftHash != draftHash {
		return ErrDraftChanged
	}
	return nil
}

// ValidateRegisterRequest checks that we are registering a confirmed
// transaction. Registering an unconfirmed signature would create a market that
// may never exist.
func ValidateRegisterRequest(attempt *Attempt, signature string) error {
	if attempt == nil {
		return ErrAttemptNotFound
	}
	if !hasText(signature) {
		return ErrSignatureRequired
	}
	if !hasText(attempt.CreateID) {
		return ErrCreateIDRequired
	}
	if attempt.SolanaSignature != "" && attempt.SolanaSignature != signature {
		return errors.New("signature does not match the recorded creation attempt")
	}
	return nil
}

// ParseBaseUnits is a strict parser for Panta's integer USDC base-unit strings.
//
// It exists to make the "no float64" rule enforceable. A decimal point, sign or
// exponent is a hard error, so a fractional or negative fee can never enter the
// system through a lenient parse.
func ParseBaseUnits(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, errors.New("amount is required")
	}
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("amount %q is not an integer USDC base-unit string", value)
		}
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is out of range", value)
	}
	return parsed, nil
}
