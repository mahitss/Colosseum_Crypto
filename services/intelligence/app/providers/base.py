"""AI Provider abstraction for Prophet Copilot."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any, Dict, List, Optional
from pydantic import BaseModel


class ToolCall(BaseModel):
    """A tool call made by the agent."""
    name: str
    arguments: Dict[str, Any]
    result: Optional[Any] = None


class AIResponse(BaseModel):
    """Response from AI provider."""
    content: str
    reasoning: Optional[str] = None
    model: str
    provider: str
    usage: Optional[Dict[str, Any]] = None
    tool_calls: Optional[List[ToolCall]] = None


class AIProvider(ABC):
    """Abstract AI provider interface."""
    
    def __init__(self, model: str, api_key: str):
        self.model = model
        self.api_key = api_key
    
    @abstractmethod
    async def generate(
        self,
        messages: List[Dict[str, str]],
        system_prompt: str,
        tools: Optional[List[Dict[str, Any]]] = None,
        max_tokens: int = 1000,
        temperature: float = 0.7,
    ) -> AIResponse:
        """Generate a response from the AI model."""
        pass
    
    @abstractmethod
    async def generate_structured(
        self,
        messages: List[Dict[str, str]],
        system_prompt: str,
        response_model: type,
        tools: Optional[List[Dict[str, Any]]] = None,
    ) -> AIResponse:
        """Generate a structured response from the AI model."""
        pass
    
    @abstractmethod
    async def close(self):
        """Close the provider connection."""
        pass
