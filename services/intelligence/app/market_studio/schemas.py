"""Strict Pydantic models for the Market Studio draft.

Design rules enforced here:

1. Only fields that the *verified* Panta create API accepts are sent upstream.
2. Panta has no per-outcome YES/NO text fields. We keep the Prophet-side
   ``outcome_yes`` / ``outcome_no`` as clearly-labelled presentation data because
   the review UX needs them, but they are never transmitted to Panta.
3. Structural problems (missing field, bad date, oversized text) are rejected by
   Pydantic. Business rules (ambiguity, expiry, source confirmation) live in
   ``validation.py`` so the LLM is never the final validator.
"""

from __future__ import annotations

from datetime import date, datetime, timezone
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, ConfigDict, Field, field_validator

# --------------------------------------------------------------------------- #
# Panta constants (verified against docs.panta.market)
# --------------------------------------------------------------------------- #

#: Panta create-quote ``category`` allowlist.
PANTA_CATEGORIES: List[str] = [
    "sports",
    "crypto",
    "politics",
    "entertainment",
    "finance",
    "science",
    "world",
    "other",
]

#: Documented field limits from the Panta create-quote endpoint. These are
#: enforced by ``validation.py`` so the user gets an actionable message.
MAX_QUESTION_LENGTH = 512
MAX_RESOLUTION_RULE_LENGTH = 2048
MAX_IMAGE_URL_LENGTH = 2048
MAX_SOURCES_OF_TRUTH = 20
MAX_OUTCOME_LENGTH = 300

#: Hard safety envelope enforced by Pydantic itself. These sit deliberately well
#: above the documented Panta limits: their only job is to reject absurd input
#: before it reaches an AI provider or the database, while the precise, user
#: facing business limits stay in deterministic validation.
HARD_INPUT_CEILING = 8192

#: Panta requires ``startTime < endTime <= resolutionTime`` and, for standard
#: markets, ``startTime`` at least ``minimumStartDelay`` ahead of now
#: (typically 3600 seconds).
DEFAULT_MINIMUM_START_DELAY_SECONDS = 3600


def _utc_today() -> date:
    return datetime.now(timezone.utc).date()


# --------------------------------------------------------------------------- #
# Market draft
# --------------------------------------------------------------------------- #


class MarketDraft(BaseModel):
    """A structured, user-editable market proposal.

    Panta-transmitted fields are the first block; Prophet-only presentation and
    review fields follow and are marked ``panta=False``.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    # ---- Panta create-quote fields ------------------------------------- #
    question: str = Field(
        ...,
        min_length=1,
        max_length=HARD_INPUT_CEILING,
        description="Market question. Panta max 512 characters.",
    )
    resolution_criteria: str = Field(
        ...,
        min_length=1,
        max_length=HARD_INPUT_CEILING,
        description="Maps to Panta `resolutionRule`. Max 2048 characters.",
    )
    sources_of_truth: List[str] = Field(
        default_factory=list,
        max_length=MAX_SOURCES_OF_TRUTH,
        description="Maps to Panta `sourcesOfTruth`. Non-empty list, max 20.",
    )
    category: str = Field(
        default="other",
        description="Panta category slug.",
    )
    resolution_date: date = Field(
        ...,
        description="Maps to Panta `resolutionTime` (converted to unix seconds).",
    )
    end_date: Optional[date] = Field(
        default=None,
        description="Maps to Panta `endTime`. Defaults to resolution_date when unset.",
    )
    start_date: Optional[date] = Field(
        default=None,
        description="Maps to Panta `startTime`. Defaults to today when unset.",
    )
    # Panta requires this field, but the assistant must not invent a URL. It is
    # therefore allowed to be blank here and blocked by deterministic validation
    # with an actionable message, rather than crashing the schema.
    image_url: str = Field(
        default="",
        max_length=HARD_INPUT_CEILING,
        description="Panta `imageUrl`. REQUIRED by Panta even though it is catalog-only.",
    )

    # ---- Optional Panta fields ----------------------------------------- #
    title: Optional[str] = Field(default=None, max_length=MAX_QUESTION_LENGTH)
    description: Optional[str] = Field(default=None, max_length=MAX_RESOLUTION_RULE_LENGTH)
    region: Optional[str] = Field(default=None, max_length=64)
    market_type: str = Field(default="standard", description="'standard' or 'breaking'.")

    # ---- Prophet-only presentation data (NEVER sent to Panta) ---------- #
    outcome_yes: Optional[str] = Field(
        default=None,
        max_length=MAX_OUTCOME_LENGTH,
        description="Prophet-only. Panta has no per-outcome text field.",
    )
    outcome_no: Optional[str] = Field(
        default=None,
        max_length=MAX_OUTCOME_LENGTH,
        description="Prophet-only. Panta has no per-outcome text field.",
    )
    notes: Optional[str] = Field(
        default=None,
        max_length=1000,
        description="Prophet-only private note. Never sent to Panta.",
    )

    # ---- Review provenance --------------------------------------------- #
    resolution_source_confirmed: bool = Field(
        default=False,
        description=(
            "False means the AI could not identify an authoritative source and the "
            "user must confirm one explicitly before creation is allowed."
        ),
    )

    # ---- Validators --------------------------------------------------- #
    # NOTE: precise Panta limits (question 512, resolutionRule 2048, category
    # allowlist, source count) are enforced in ``validation.py`` so the user
    # receives an actionable, field-level message. Pydantic only guarantees the
    # shape is structurally parseable, plus a hard safety ceiling on size.

    @field_validator("market_type")
    @classmethod
    def _validate_market_type(cls, value: str) -> str:
        normalized = value.strip().lower()
        if normalized not in ("standard", "breaking"):
            raise ValueError("market_type must be 'standard' or 'breaking'")
        return normalized

    @field_validator("sources_of_truth")
    @classmethod
    def _validate_sources(cls, value: List[str]) -> List[str]:
        cleaned = [item.strip() for item in value if item and item.strip()]
        if len(cleaned) > MAX_SOURCES_OF_TRUTH:
            raise ValueError(f"at most {MAX_SOURCES_OF_TRUTH} sources are allowed")
        for item in cleaned:
            if len(item) > 512:
                raise ValueError("each source of truth must be 512 characters or fewer")
        return cleaned

    # ---- Derived helpers ---------------------------------------------- #

    def effective_start_date(self) -> date:
        return self.start_date or _utc_today()

    def effective_end_date(self) -> date:
        return self.end_date or self.resolution_date

    def to_panta_create_params(self) -> Dict[str, Any]:
        """Build the exact Panta create-quote body for this draft.

        Only verified Panta fields are included. Prophet-only fields such as
        ``outcome_yes``/``outcome_no``/``notes`` are intentionally absent.
        """

        def _midnight_epoch(day: date) -> int:
            return int(
                datetime(day.year, day.month, day.day, tzinfo=timezone.utc).timestamp()
            )

        # Panta requires startTime < endTime <= resolutionTime. When the user only
        # supplies a resolution date we widen the window deterministically so the
        # ordering constraint always holds.
        start = _midnight_epoch(self.effective_start_date())
        end = _midnight_epoch(self.effective_end_date())
        resolution = _midnight_epoch(self.resolution_date)

        if start >= end:
            end = start + 86400
        if end > resolution:
            resolution = end

        body: Dict[str, Any] = {
            "question": self.question,
            "resolutionRule": self.resolution_criteria,
            "sourcesOfTruth": list(self.sources_of_truth),
            "category": self.category,
            "startTime": start,
            "endTime": end,
            "resolutionTime": resolution,
            "imageUrl": self.image_url,
            "marketType": self.market_type,
        }
        if self.title:
            body["title"] = self.title
        if self.description:
            body["description"] = self.description
        if self.region:
            body["region"] = self.region
        return body


# --------------------------------------------------------------------------- #
# Interpretation result
# --------------------------------------------------------------------------- #


class MarketInterpretation(BaseModel):
    """Result of turning natural language into a draft.

    ``needs_clarification`` is the signal that the user's question was too
    ambiguous to become a market. We report the gaps instead of inventing them.
    """

    model_config = ConfigDict(extra="forbid")

    draft: Optional[MarketDraft] = None
    needs_clarification: bool = False
    missing_fields: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)
    #: True when no AI provider was configured and a deterministic fallback ran.
    used_fallback: bool = False


# --------------------------------------------------------------------------- #
# Deterministic validation result
# --------------------------------------------------------------------------- #


class ValidationIssue(BaseModel):
    """One deterministic validation finding."""

    model_config = ConfigDict(extra="forbid")

    field: str
    code: str
    message: str
    #: "error" blocks creation; "warning" must be shown but does not block.
    severity: str = "error"


class ValidationReport(BaseModel):
    """Outcome of deterministic validation. The LLM is never consulted here."""

    model_config = ConfigDict(extra="forbid")

    valid: bool
    issues: List[ValidationIssue] = Field(default_factory=list)
    missing_fields: List[str] = Field(default_factory=list)
    needs_clarification: bool = False
    #: Short human-readable fingerprint of the reviewed content.
    draft_fingerprint: Optional[str] = None

    @property
    def errors(self) -> List[ValidationIssue]:
        return [issue for issue in self.issues if issue.severity == "error"]

    @property
    def warnings(self) -> List[ValidationIssue]:
        return [issue for issue in self.issues if issue.severity == "warning"]
