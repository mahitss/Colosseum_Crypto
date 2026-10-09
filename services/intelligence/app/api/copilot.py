"""API endpoints for Prophet Copilot."""

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from app.agent.agent import CopilotAgent, create_agent
from app.agent.prompts import SYSTEM_PROMPT

logger = logging.getLogger(__name__)

router = APIRouter()


class QueryRequest(BaseModel):
    """Request body for copilot query."""
    message: str = Field(..., min_length=1, max_length=1000)
    conversation_id: Optional[str] = Field(default=None, max_length=36)


class Source(BaseModel):
    """Source reference for a response."""
    type: str
    id: Optional[str] = None
    title: Optional[str] = None
    market_id: Optional[str] = None
    source: str = "prophet_database"


class QueryResponse(BaseModel):
    """Response from copilot query."""
    answer: str
    sources: list[Source] = []
    intent: str = "GENERAL_QUERY"
    tool_calls: list[dict] = []
    generated_at: str


@router.post("/copilot/query", response_model=QueryResponse)
async def copilot_query(request: QueryRequest):
    """
    Process a copilot query.
    
    Args:
        request: QueryRequest containing the user message
        
    Returns:
        QueryResponse with answer and sources
    """
    try:
        # Create agent
        agent = create_agent()
        
        # Process query
        response = await agent.query(
            user_message=request.message,
            conversation_id=request.conversation_id,
        )
        
        # Clean up
        await agent.close()
        
        return QueryResponse(
            answer=response.answer,
            sources=response.sources,
            intent=response.intent,
            tool_calls=response.tool_calls,
            generated_at=response.generated_at.isoformat(),
        )
        
    except Exception as e:
        logger.error("copilot query failed", extra={"error": str(e)})
        raise HTTPException(status_code=500, detail="Error processing query")


@router.get("/copilot/health")
async def copilot_health():
    """Health check for copilot service."""
    return {
        "status": "ok",
        "service": "prophet-copilot",
    }
