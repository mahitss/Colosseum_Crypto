"""The AI Market Architect.

This agent only *structures* a market. It has no capability to create, quote,
sign, broadcast, or register anything. Its output is always re-validated by
``validation.validate_draft`` before it can reach a human review screen.

The agent is deliberately separate from the read-only Copilot agent in
``app.agent`` so that market-creation concerns are never mixed into the Copilot.
"""

from __future__ import annotations

import json
import logging
import re
from typing import Any, Dict, List, Optional

from app.config import settings
from app.market_studio.prompts import SYSTEM_PROMPT, build_user_prompt
from app.market_studio.schemas import (
    MarketDraft,
    MarketInterpretation,
    PANTA_CATEGORIES,
)
from app.market_studio.validation import (
    collect_missing_fields,
    is_ambiguous_prompt,
    summarize_missing,
    validate_draft,
)
from app.providers.openai import OpenAIProvider

logger = logging.getLogger(__name__)

#: Guardrail: refuse absurdly long payloads before they reach a provider.
MAX_DESCRIPTION_LENGTH = 4000

#: Guardrail: a model that rambles must not be allowed to blow up parsing.
MAX_MODEL_OUTPUT_LENGTH = 20000

_JSON_BLOCK = re.compile(r"\{.*\}", re.DOTALL)


class MarketStudioError(RuntimeError):
    """Raised when interpretation cannot be completed."""

    code = "AI_INTERPRETATION_FAILED"


def _strip_code_fences(text: str) -> str:
    cleaned = text.strip()
    if cleaned.startswith("```"):
        cleaned = re.sub(r"^```[a-zA-Z]*\n?", "", cleaned)
        if cleaned.rstrip().endswith("```"):
            cleaned = cleaned.rstrip()[:-3]
    return cleaned.strip()


def _extract_json(text: str) -> Optional[Dict[str, Any]]:
    """Pull the first well-formed JSON object out of a model response."""

    if not text:
        return None
    candidate = _strip_code_fences(text[:MAX_MODEL_OUTPUT_LENGTH])
    try:
        parsed = json.loads(candidate)
        return parsed if isinstance(parsed, dict) else None
    except json.JSONDecodeError:
        pass

    match = _JSON_BLOCK.search(candidate)
    if not match:
        return None
    try:
        parsed = json.loads(match.group(0))
    except json.JSONDecodeError:
        return None
    return parsed if isinstance(parsed, dict) else None


#: Keys the model is allowed to influence. Anything else is discarded so a
#: confused or hostile response cannot inject unexpected state.
_ALLOWED_DRAFT_KEYS = {
    "question",
    "description",
    "outcome_yes",
    "outcome_no",
    "resolution_criteria",
    "sources_of_truth",
    "resolution_source_confirmed",
    "category",
    "resolution_date",
    "end_date",
    "start_date",
    "image_url",
    "title",
    "region",
    "market_type",
    "notes",
}


def _coerce_draft_payload(raw: Dict[str, Any]) -> Dict[str, Any]:
    """Filter, normalise and clamp model output into a draft payload.

    Model output is untrusted: unknown keys are dropped, an empty or missing
    ``sources_of_truth`` is normalised to an empty list, and a claimed
    confirmation is only honoured when a real source is present.
    """

    payload = {key: value for key, value in raw.items() if key in _ALLOWED_DRAFT_KEYS}

    sources = payload.get("sources_of_truth")
    if isinstance(sources, str):
        sources = [sources]
    if not isinstance(sources, list):
        sources = []
    payload["sources_of_truth"] = [
        str(item).strip() for item in sources if str(item).strip()
    ][:20]

    # The assistant must NEVER mark the resolution source as confirmed. It has
    # no way to know whether the user actually named this source or whether the
    # model just invented something plausible, and a self-reported "true" from
    # untrusted model output is not evidence of anything.
    #
    # Confirmation is a human act, performed by ticking the checkbox in the
    # wizard. The prompt asks the model to flag when a source is ambiguous, but
    # that is a request for clarification, not permission to self-authorise:
    # forcing this to False here is what makes the rule structural rather than
    # a matter of the model cooperating.
    payload["resolution_source_confirmed"] = False

    category = str(payload.get("category", "other")).strip().lower()
    payload["category"] = category if category in PANTA_CATEGORIES else "other"

    if not str(payload.get("image_url", "")).strip():
        # An invented placeholder URL would be worse than an honest blank that
        # validation blocks on, because the human must consciously supply it.
        payload["image_url"] = ""

    for optional_key in ("outcome_yes", "outcome_no", "notes", "title", "region", "description"):
        if payload.get(optional_key) is not None:
            payload[optional_key] = str(payload[optional_key]).strip()

    payload.setdefault("market_type", "standard")
    return payload


def _deterministic_fallback(description: str) -> MarketInterpretation:
    """Structure a draft without any AI provider.

    Used when no AI key is configured, in tests, and as the safety net when the
    provider returns malformed output. It never invents a source and never
    invents a deadline that is not present in the text.
    """

    if is_ambiguous_prompt(description):
        message, missing = summarize_missing(
            ["deadline", "threshold_or_yes_definition", "resolution_criteria", "resolution_source"]
        )
        return MarketInterpretation(
            draft=None,
            needs_clarification=True,
            missing_fields=missing,
            warnings=[message],
            used_fallback=True,
        )

    return MarketInterpretation(
        draft=None,
        needs_clarification=True,
        missing_fields=[
            "question",
            "outcome_yes",
            "outcome_no",
            "resolution_criteria",
            "resolution_source",
            "resolution_source_confirmation",
            "resolution_date",
            "image_url",
        ],
        warnings=[
            "No AI provider is configured, so no draft was generated. "
            "Fill in the market fields manually."
        ],
        used_fallback=True,
    )


class MarketStudioAgent:
    """Converts natural language into a structured, validated market draft."""

    def __init__(self, provider: Optional[Any] = None) -> None:
        self._provider = provider
        self._owns_provider = provider is None

    def _build_provider(self) -> Optional[Any]:
        if not settings.ai_api_key:
            return None
        try:
            return OpenAIProvider(model="gpt-4o-mini", api_key=settings.ai_api_key)
        except Exception:  # pragma: no cover - provider import/runtime issues
            logger.warning("AI provider unavailable; falling back", exc_info=True)
            return None

    async def interpret(self, description: str) -> MarketInterpretation:
        """Structure a market from a natural-language description.

        This method never creates anything. It returns a draft plus the
        deterministic validation verdict so the UI can show every warning.
        """

        cleaned = (description or "").strip()
        if not cleaned:
            raise MarketStudioError("description is required")
        if len(cleaned) > MAX_DESCRIPTION_LENGTH:
            raise MarketStudioError(
                f"description must be {MAX_DESCRIPTION_LENGTH} characters or fewer"
            )

        provider = self._provider or self._build_provider()
        if provider is None:
            return _deterministic_fallback(cleaned)

        try:
            response = await provider.generate(
                messages=[{"role": "user", "content": build_user_prompt(cleaned)}],
                system_prompt=SYSTEM_PROMPT,
                max_tokens=1500,
                # Low temperature: we want faithful structuring, not creativity.
                temperature=0.1,
            )
        except Exception as exc:  # noqa: BLE001 - never surface provider internals
            logger.warning("AI interpretation failed: %s", type(exc).__name__)
            raise MarketStudioError("AI interpretation failed") from exc

        payload = _extract_json(getattr(response, "content", "") or "")
        if payload is None:
            # Malformed model output must never become a market draft.
            return MarketInterpretation(
                draft=None,
                needs_clarification=True,
                missing_fields=[
                    "question",
                    "resolution_criteria",
                    "resolution_source",
                    "resolution_date",
                ],
                warnings=[
                    "The assistant returned an unreadable response. "
                    "Please rephrase, or fill in the market fields manually."
                ],
            )

        needs_clarification = bool(payload.get("needs_clarification", False))
        raw_warnings = payload.get("warnings")
        warnings: List[str] = [
            str(item) for item in raw_warnings if str(item).strip()
        ] if isinstance(raw_warnings, list) else []

        raw_draft = payload.get("draft")
        if needs_clarification or not isinstance(raw_draft, dict):
            raw_missing = payload.get("missing_fields")
            missing = (
                [str(item) for item in raw_missing]
                if isinstance(raw_missing, list) and raw_missing
                else ["question", "resolution_criteria", "resolution_source", "resolution_date"]
            )
            message, missing = summarize_missing(missing)
            return MarketInterpretation(
                draft=None,
                needs_clarification=True,
                missing_fields=missing,
                warnings=[message, *warnings],
            )

        draft_payload = _coerce_draft_payload(raw_draft)
        try:
            draft = MarketDraft(**draft_payload)
        except Exception as exc:  # noqa: BLE001
            # Pydantic is the final structural gate; malformed drafts never pass.
            logger.info("draft failed schema validation: %s", type(exc).__name__)
            message, missing = summarize_missing(
                collect_missing_fields(None) or [
                    "question",
                    "resolution_criteria",
                    "resolution_source",
                    "resolution_date",
                    "image_url",
                ]
            )
            return MarketInterpretation(
                draft=None,
                needs_clarification=True,
                missing_fields=missing,
                warnings=[
                    "The draft produced by the assistant did not satisfy the market "
                    "schema and was discarded.",
                    *warnings,
                ],
            )

        report = validate_draft(draft, prompt=cleaned)
        for issue in report.issues:
            if issue.severity == "warning":
                warnings.append(issue.message)

        return MarketInterpretation(
            draft=draft,
            needs_clarification=False,
            missing_fields=report.missing_fields,
            warnings=warnings,
        )

    async def close(self) -> None:
        if self._owns_provider and self._provider is not None:
            try:
                await self._provider.close()
            except Exception:  # pragma: no cover - best-effort cleanup
                pass
