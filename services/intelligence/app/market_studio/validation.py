"""Deterministic validation for market drafts.

This module is the gate that decides whether a market may proceed to a Panta
creation quote. It never calls an LLM. AI output must pass here before the user
is even offered a "Create Market" action.

Rules implemented:

* structural sanity (question present, usable length, no control characters)
* outcome coverage (YES/NO definitions present for review)
* resolution criteria, source and date present
* source-of-truth explicitly confirmed when the AI could not verify one
* date is valid, not expired, and honours Panta's ordering constraint
* Panta field limits and the category allowlist

Ambiguity is reported, never auto-filled.
"""

from __future__ import annotations

import re
from datetime import date, datetime, timezone
from typing import List, Optional, Tuple

from app.market_studio.schemas import (
    DEFAULT_MINIMUM_START_DELAY_SECONDS,
    MAX_IMAGE_URL_LENGTH,
    MAX_OUTCOME_LENGTH,
    MAX_QUESTION_LENGTH,
    MAX_RESOLUTION_RULE_LENGTH,
    MAX_SOURCES_OF_TRUTH,
    PANTA_CATEGORIES,
    MarketDraft,
    ValidationIssue,
    ValidationReport,
)

#: Minimum sensible resolution window. A market that resolves the same day it is
#: created cannot be meaningfully traded, and Panta requires a start delay.
MIN_RESOLUTION_DAYS = 1

_CONTROL_CHARS = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]")

#: Phrases that indicate a resolution source the AI should not silently accept.
_UNVERIFIED_SOURCE_MARKERS = (
    "tbd",
    "t.b.d",
    "to be determined",
    "unknown",
    "n/a",
    "na",
    "some source",
    "appropriate source",
    "the source",
    "any source",
    "trusted source",
    "officially",
    "official source",
)

#: Prompts that attempt to steer or override the architect.
_INJECTION_MARKERS = (
    "ignore previous",
    "ignore all previous",
    "disregard the system",
    "system prompt",
    "you are now",
    "act as",
    "jailbreak",
    "developer mode",
    "reveal your",
    "print your prompt",
    "auto-approve",
    "approve the market",
    "skip validation",
    "bypass validation",
    "create the market for me",
    "sign the transaction",
    "without user confirmation",
)


def _today() -> date:
    return datetime.now(timezone.utc).date()


def _has_usable_content(value: Optional[str], minimum: int = 3) -> bool:
    if value is None:
        return False
    stripped = value.strip()
    if len(stripped) < minimum:
        return False
    # A string made only of punctuation is not real content.
    return any(character.isalnum() for character in stripped)


def is_ambiguous_prompt(prompt: str) -> bool:
    """Detect obviously under-specified questions such as "Will BTC go up?".

    A prompt is treated as ambiguous when it has no resolvable deadline and no
    numeric threshold, which is the combination that makes a binary market
    undecidable.
    """

    normalized = " ".join(prompt.lower().split())
    if not normalized:
        return True

    has_deadline = bool(
        re.search(
            r"\b(by|before|after|on|until|through|end of|eod|deadline)\b"
            r"|\b(20\d{2})\b"
            r"|\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\b"
            r"|\b\d{1,2}(am|pm)\b",
            normalized,
        )
    )
    has_threshold = bool(re.search(r"[\$€£]?\d", normalized))

    if not has_deadline and not has_threshold:
        return True
    return False


def detect_prompt_injection(prompt: str) -> Optional[str]:
    """Return the offending marker when a prompt looks like an injection attempt."""

    normalized = " ".join(prompt.lower().split())
    for marker in _INJECTION_MARKERS:
        if marker in normalized:
            return marker
    return None


def _validate_question(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    question = (draft.question or "").strip()

    if not _has_usable_content(question, minimum=10):
        issues.append(
            ValidationIssue(
                field="question",
                code="QUESTION_UNCLEAR",
                message="The question must state a specific, checkable claim.",
            )
        )
        return issues

    if _CONTROL_CHARS.search(question):
        issues.append(
            ValidationIssue(
                field="question",
                code="QUESTION_INVALID_CHARACTERS",
                message="The question contains unsupported control characters.",
            )
        )
    if len(question) > MAX_QUESTION_LENGTH:
        issues.append(
            ValidationIssue(
                field="question",
                code="QUESTION_TOO_LONG",
                message=(
                    f"The question must be {MAX_QUESTION_LENGTH} characters or fewer "
                    f"(currently {len(question)})."
                ),
            )
        )
    if question.endswith("?"):
        issues.append(
            ValidationIssue(
                field="question",
                code="QUESTION_NOT_A_CLAIM",
                message=(
                    "The question must be a declarative claim, not a question. "
                    "Try: 'ETH will be above $5,000 on December 31, 2026'."
                ),
            )
        )
    return issues


def _validate_outcomes(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    yes = (draft.outcome_yes or "").strip()
    no = (draft.outcome_no or "").strip()

    if not _has_usable_content(yes):
        issues.append(
            ValidationIssue(
                field="outcome_yes",
                code="OUTCOME_YES_MISSING",
                message="Define what resolves YES.",
            )
        )
    elif len(yes) > MAX_OUTCOME_LENGTH:
        issues.append(
            ValidationIssue(
                field="outcome_yes",
                code="OUTCOME_YES_TOO_LONG",
                message=f"The YES outcome must be {MAX_OUTCOME_LENGTH} characters or fewer.",
            )
        )

    if not _has_usable_content(no):
        issues.append(
            ValidationIssue(
                field="outcome_no",
                code="OUTCOME_NO_MISSING",
                message="Define what resolves NO.",
            )
        )
    elif len(no) > MAX_OUTCOME_LENGTH:
        issues.append(
            ValidationIssue(
                field="outcome_no",
                code="OUTCOME_NO_TOO_LONG",
                message=f"The NO outcome must be {MAX_OUTCOME_LENGTH} characters or fewer.",
            )
        )
    return issues


def _validate_resolution_criteria(draft: MarketDraft) -> List[ValidationIssue]:
    criteria = (draft.resolution_criteria or "").strip()
    if not _has_usable_content(criteria, minimum=10):
        return [
            ValidationIssue(
                field="resolution_criteria",
                code="RESOLUTION_CRITERIA_MISSING",
                message=(
                    "Resolution criteria are required. State exactly how the market "
                    "settles and which value is observed."
                ),
            )
        ]

    issues: List[ValidationIssue] = []
    if len(criteria) > MAX_RESOLUTION_RULE_LENGTH:
        issues.append(
            ValidationIssue(
                field="resolution_criteria",
                code="RESOLUTION_CRITERIA_TOO_LONG",
                message=(
                    f"Resolution criteria must be {MAX_RESOLUTION_RULE_LENGTH} "
                    f"characters or fewer (currently {len(criteria)})."
                ),
            )
        )
    if _CONTROL_CHARS.search(criteria):
        issues.append(
            ValidationIssue(
                field="resolution_criteria",
                code="RESOLUTION_CRITERIA_INVALID_CHARACTERS",
                message="Resolution criteria contain unsupported control characters.",
            )
        )
    return issues


def _validate_sources(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    sources = [item for item in draft.sources_of_truth if item and item.strip()]

    if not sources:
        # Missing source is blocking, but we phrase it as a confirmation request
        # because the user may legitimately supply one.
        issues.append(
            ValidationIssue(
                field="sources_of_truth",
                code="RESOLUTION_SOURCE_MISSING",
                message=(
                    "A resolution source is required. Add at least one authoritative "
                    "source used to settle this market."
                ),
            )
        )
        return issues

    if len(sources) > MAX_SOURCES_OF_TRUTH:
        issues.append(
            ValidationIssue(
                field="sources_of_truth",
                code="RESOLUTION_SOURCE_LIMIT",
                message=f"At most {MAX_SOURCES_OF_TRUTH} sources are allowed.",
            )
        )

    for source in sources:
        lowered = source.strip().lower().rstrip(".")
        if not source.strip():
            issues.append(
                ValidationIssue(
                    field="sources_of_truth",
                    code="RESOLUTION_SOURCE_EMPTY",
                    message="A resolution source entry is empty.",
                )
            )
            continue
        if any(lowered == marker or lowered.startswith(marker) for marker in _UNVERIFIED_SOURCE_MARKERS):
            issues.append(
                ValidationIssue(
                    field="sources_of_truth",
                    code="RESOLUTION_SOURCE_UNVERIFIED",
                    message=(
                        f"'{source}' is not a specific authoritative source. Name the "
                        "exact publication or feed used for settlement."
                    ),
                )
            )
        if len(source) > 512:
            issues.append(
                ValidationIssue(
                    field="sources_of_truth",
                    code="RESOLUTION_SOURCE_TOO_LONG",
                    message="Each resolution source must be 512 characters or fewer.",
                )
            )
    return issues


def _validate_source_confirmation(draft: MarketDraft) -> List[ValidationIssue]:
    """An AI-suggested source still needs explicit human confirmation."""

    if draft.resolution_source_confirmed:
        return []
    return [
        ValidationIssue(
            field="resolution_source_confirmed",
            code="RESOLUTION_SOURCE_REQUIRES_CONFIRMATION",
            message=(
                "Confirm the resolution source before this market can be created. "
                "The assistant proposed it; you decide it."
            ),
            severity="error",
        )
    ]


def _validate_dates(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    today = _today()

    try:
        resolution = draft.resolution_date
    except (TypeError, ValueError):
        return [
            ValidationIssue(
                field="resolution_date",
                code="RESOLUTION_DATE_INVALID",
                message="The resolution date is not a valid date.",
            )
        ]

    if resolution <= today:
        issues.append(
            ValidationIssue(
                field="resolution_date",
                code="RESOLUTION_DATE_EXPIRED",
                message=(
                    f"The resolution date ({resolution.isoformat()}) is not in the "
                    "future. Choose a future date."
                ),
            )
        )
    elif (resolution - today).days < MIN_RESOLUTION_DAYS:
        issues.append(
            ValidationIssue(
                field="resolution_date",
                code="RESOLUTION_DATE_TOO_SOON",
                message=(
                    "The resolution date is too close. Panta requires a minimum start "
                    "delay before trading opens, so allow at least one day."
                ),
            )
        )

    start = draft.effective_start_date()
    end = draft.effective_end_date()

    if start < today:
        issues.append(
            ValidationIssue(
                field="start_date",
                code="START_DATE_PAST",
                message="The start date cannot be in the past.",
            )
        )
    if not (start < end <= resolution):
        issues.append(
            ValidationIssue(
                field="end_date",
                code="DATE_ORDER_INVALID",
                message=(
                    "Dates must satisfy start < end <= resolution "
                    f"(got {start.isoformat()} / {end.isoformat()} / "
                    f"{resolution.isoformat()})."
                ),
            )
        )

    if (
        draft.market_type == "standard"
        and resolution > today
        and (resolution - today).days * 86400 < DEFAULT_MINIMUM_START_DELAY_SECONDS
    ):
        issues.append(
            ValidationIssue(
                field="resolution_date",
                code="MINIMUM_START_DELAY",
                message=(
                    f"Panta requires at least "
                    f"{DEFAULT_MINIMUM_START_DELAY_SECONDS} seconds of start delay for "
                    "standard markets."
                ),
            )
        )
    return issues


def _validate_image(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    url = (draft.image_url or "").strip()

    if not url:
        issues.append(
            ValidationIssue(
                field="image_url",
                code="IMAGE_URL_MISSING",
                message=(
                    "Panta requires a catalog image URL. Provide a publicly reachable "
                    "http(s) image."
                ),
            )
        )
        return issues

    if len(url) > MAX_IMAGE_URL_LENGTH:
        issues.append(
            ValidationIssue(
                field="image_url",
                code="IMAGE_URL_TOO_LONG",
                message=f"The image URL must be {MAX_IMAGE_URL_LENGTH} characters or fewer.",
            )
        )
    if not url.lower().startswith(("http://", "https://")):
        issues.append(
            ValidationIssue(
                field="image_url",
                code="IMAGE_URL_INVALID",
                message="The image URL must start with http:// or https://.",
            )
        )
    if url.lower().startswith("data:"):
        issues.append(
            ValidationIssue(
                field="image_url",
                code="IMAGE_URL_DATA_UNSUPPORTED",
                message="Data URLs are not accepted. Host the image and pass its URL.",
            )
        )
    lowered = url.lower()
    for host_marker in ("localhost", "127.0.0.1", "0.0.0.0", "::1", "10.", "192.168.", "172.16."):
        if host_marker in lowered:
            issues.append(
                ValidationIssue(
                    field="image_url",
                    code="IMAGE_URL_PRIVATE_HOST",
                    message=(
                        "The image must be publicly reachable; private and localhost "
                        "hosts are rejected by Panta."
                    ),
                )
            )
            break
    return issues


def _validate_panta_constraints(draft: MarketDraft) -> List[ValidationIssue]:
    issues: List[ValidationIssue] = []
    if draft.category not in PANTA_CATEGORIES:
        issues.append(
            ValidationIssue(
                field="category",
                code="CATEGORY_INVALID",
                message=(
                    "Category must be one of: " + ", ".join(PANTA_CATEGORIES) + "."
                ),
            )
        )
    if draft.market_type not in ("standard", "breaking"):
        issues.append(
            ValidationIssue(
                field="market_type",
                code="MARKET_TYPE_INVALID",
                message="Market type must be 'standard' or 'breaking'.",
            )
        )
    if draft.title and len(draft.title) > MAX_QUESTION_LENGTH:
        issues.append(
            ValidationIssue(
                field="title",
                code="TITLE_TOO_LONG",
                message=f"The title must be {MAX_QUESTION_LENGTH} characters or fewer.",
            )
        )
    if draft.description and len(draft.description) > MAX_RESOLUTION_RULE_LENGTH:
        issues.append(
            ValidationIssue(
                field="description",
                code="DESCRIPTION_TOO_LONG",
                message=(
                    f"The description must be {MAX_RESOLUTION_RULE_LENGTH} characters "
                    "or fewer."
                ),
            )
        )
    return issues


def collect_missing_fields(draft: Optional[MarketDraft], prompt: str = "") -> List[str]:
    """Names of the fields a human still has to supply."""

    missing: List[str] = []
    if draft is None:
        return missing

    if not _has_usable_content(draft.question):
        missing.append("question")
    if not _has_usable_content(draft.outcome_yes):
        missing.append("threshold_or_yes_definition")
    if not _has_usable_content(draft.outcome_no):
        missing.append("no_definition")
    if not _has_usable_content(draft.resolution_criteria):
        missing.append("resolution_criteria")
    if not [item for item in draft.sources_of_truth if item and item.strip()]:
        missing.append("resolution_source")
    if not draft.resolution_source_confirmed:
        missing.append("resolution_source_confirmation")
    if draft.resolution_date is None:
        missing.append("deadline")
    elif draft.resolution_date <= _today():
        missing.append("future_deadline")
    if not (draft.image_url or "").strip():
        missing.append("image_url")
    return missing


def validate_draft(draft: Optional[MarketDraft], prompt: str = "") -> ValidationReport:
    """Run every deterministic rule and return a complete report.

    ``prompt`` is optional and only used to report prompt-injection attempts.
    """

    issues: List[ValidationIssue] = []

    if prompt:
        marker = detect_prompt_injection(prompt)
        if marker:
            issues.append(
                ValidationIssue(
                    field="prompt",
                    code="PROMPT_INJECTION_DETECTED",
                    message=(
                        "The description contained instructions that try to control the "
                        "assistant. Those were ignored; the draft was structured only "
                        "from the market content."
                    ),
                    severity="warning",
                )
            )

    if draft is None:
        issues.append(
            ValidationIssue(
                field="draft",
                code="INVALID_DRAFT",
                message="No market draft was produced from that description.",
            )
        )
        return ValidationReport(
            valid=False,
            issues=issues,
            missing_fields=["question", "deadline", "resolution_criteria", "resolution_source"],
            needs_clarification=True,
        )

    issues.extend(_validate_question(draft))
    issues.extend(_validate_outcomes(draft))
    issues.extend(_validate_resolution_criteria(draft))
    issues.extend(_validate_sources(draft))
    issues.extend(_validate_source_confirmation(draft))
    issues.extend(_validate_dates(draft))
    issues.extend(_validate_image(draft))
    issues.extend(_validate_panta_constraints(draft))

    errors = [issue for issue in issues if issue.severity == "error"]
    return ValidationReport(
        valid=not errors,
        issues=issues,
        missing_fields=collect_missing_fields(draft, prompt),
        needs_clarification=False,
    )


def summarize_missing(missing: List[str]) -> Tuple[str, List[str]]:
    """Human-readable clarification message plus the structured field list."""

    if not missing:
        return ("", [])

    readable = {
        "question": "a clear, checkable question",
        "threshold_or_yes_definition": "the threshold or condition that resolves YES",
        "no_definition": "what resolves NO",
        "resolution_criteria": "resolution criteria (how the market settles)",
        "resolution_source": "an authoritative resolution source",
        "resolution_source_confirmation": "your confirmation of the resolution source",
        "deadline": "a deadline",
        "future_deadline": "a future deadline",
        "image_url": "a catalog image URL required by Panta",
    }
    labels = [readable.get(item, item) for item in missing]
    return (
        "Your question needs more detail before a market can be created. "
        "Suggested missing fields: " + ", ".join(labels) + ".",
        list(missing),
    )
