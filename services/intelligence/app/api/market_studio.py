"""Market Studio API endpoints.

The AI endpoint is read-only with respect to the world: interpreting a
description never creates, quotes, signs, or registers anything. Validation is
deterministic and performed here so the frontend can show every warning.
"""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field, ValidationError

from app.market_studio.agent import MarketStudioAgent, MarketStudioError
from app.market_studio.schemas import MarketDraft, ValidationReport
from app.market_studio.validation import validate_draft

router = APIRouter()


# --------------------------------------------------------------------------- #
# Request / response DTOs (Prophet domain — never raw Panta schemas)
# --------------------------------------------------------------------------- #


class InterpretRequest(BaseModel):
    prompt: str = Field(..., min_length=1, max_length=4000)


class DraftDTO(BaseModel):
    """Market draft as exposed to the frontend.

    Mirrors the strict draft schema. The frontend edits every field here before
    anything is quoted or built.
    """

    model_config = {"extra": "forbid"}

    question: str
    description: Optional[str] = None
    outcome_yes: Optional[str] = None
    outcome_no: Optional[str] = None
    resolution_criteria: str
    sources_of_truth: List[str] = Field(default_factory=list)
    resolution_source_confirmed: bool = False
    category: str = "other"
    resolution_date: str
    end_date: Optional[str] = None
    start_date: Optional[str] = None
    image_url: str
    title: Optional[str] = None
    region: Optional[str] = None
    market_type: str = "standard"
    notes: Optional[str] = None


class ValidationIssueDTO(BaseModel):
    field: str
    code: str
    message: str
    severity: str = "error"


class InterpretResponse(BaseModel):
    draft: Optional[DraftDTO] = None
    needs_clarification: bool = False
    missing_fields: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)
    #: Deterministic validation of whatever draft came back (may be null).
    validation: Optional[Dict[str, Any]] = None


class ValidateRequest(BaseModel):
    draft: Dict[str, Any]


class ValidateResponse(BaseModel):
    valid: bool
    issues: List[ValidationIssueDTO] = Field(default_factory=list)
    missing_fields: List[str] = Field(default_factory=list)
    needs_clarification: bool = False


# --------------------------------------------------------------------------- #
# Helpers
# --------------------------------------------------------------------------- #


def _draft_to_dto(draft: MarketDraft) -> DraftDTO:
    return DraftDTO(
        question=draft.question,
        description=draft.description,
        outcome_yes=draft.outcome_yes,
        outcome_no=draft.outcome_no,
        resolution_criteria=draft.resolution_criteria,
        sources_of_truth=list(draft.sources_of_truth),
        resolution_source_confirmed=draft.resolution_source_confirmed,
        category=draft.category,
        resolution_date=draft.resolution_date.isoformat(),
        end_date=draft.end_date.isoformat() if draft.end_date else None,
        start_date=draft.start_date.isoformat() if draft.start_date else None,
        image_url=draft.image_url,
        title=draft.title,
        region=draft.region,
        market_type=draft.market_type,
        notes=draft.notes,
    )


def _report_to_dict(report: ValidationReport) -> Dict[str, Any]:
    return {
        "valid": report.valid,
        "issues": [issue.model_dump() for issue in report.issues],
        "missing_fields": report.missing_fields,
        "needs_clarification": report.needs_clarification,
    }


# --------------------------------------------------------------------------- #
# Endpoints
# --------------------------------------------------------------------------- #


@router.post("/market-studio/interpret", response_model=InterpretResponse)
async def market_studio_interpret(request: InterpretRequest):
    """Turn a natural-language description into a structured market draft.

    This endpoint creates nothing. It only structures and validates.
    """

    agent = MarketStudioAgent()
    try:
        interpretation = await agent.interpret(request.prompt)
    except MarketStudioError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    finally:
        await agent.close()

    validation_payload: Optional[Dict[str, Any]] = None
    if interpretation.draft is not None:
        validation_payload = _report_to_dict(
            validate_draft(interpretation.draft, prompt=request.prompt)
        )

    return InterpretResponse(
        draft=_draft_to_dto(interpretation.draft) if interpretation.draft else None,
        needs_clarification=interpretation.needs_clarification,
        missing_fields=interpretation.missing_fields,
        warnings=interpretation.warnings,
        validation=validation_payload,
    )


@router.post("/market-studio/validate", response_model=ValidateResponse)
async def market_studio_validate(request: ValidateRequest):
    """Deterministically validate a user-edited draft.

    No LLM is involved. Creation is blocked while this returns ``valid: false``.
    """

    try:
        draft = MarketDraft(**request.draft)
    except ValidationError as exc:
        issues = []
        for error in exc.errors():
            location = ".".join(str(part) for part in error.get("loc", ())) or "draft"
            issues.append(
                {
                    "field": location,
                    "code": "SCHEMA_INVALID",
                    "message": error.get("msg", "invalid value"),
                    "severity": "error",
                }
            )
        return ValidateResponse(valid=False, issues=issues, missing_fields=[])

    report = validate_draft(draft)
    return ValidateResponse(
        valid=report.valid,
        issues=[issue.model_dump() for issue in report.issues],
        missing_fields=report.missing_fields,
        needs_clarification=report.needs_clarification,
    )
