"""Prophet AI Copilot Agent."""

import os
from typing import Any, Dict, List, Optional
from datetime import datetime

from pydantic import BaseModel

from app.providers.base import AIProvider
from app.providers.openai import OpenAIProvider
from app.agent.tools import AgentTools, ToolResult
from app.agent.prompts import SYSTEM_PROMPT, detect_intent


class CopilotMessage(BaseModel):
    """A message in the conversation."""
    role: str  # user, assistant, tool
    content: str
    timestamp: datetime


class CopilotResponse(BaseModel):
    """Response from the Copilot."""
    answer: str
    sources: List[Dict[str, Any]] = []
    intent: str = "GENERAL_QUERY"
    tool_calls: List[Dict[str, Any]] = []
    generated_at: datetime


class CopilotAgent:
    """Prophet AI Copilot Agent."""
    
    MAX_TOOL_CALLS = 8
    MAX_HISTORY_MESSAGES = 10
    
    def __init__(self, provider: AIProvider, tools: AgentTools):
        self.provider = provider
        self.tools = tools
        self.conversation_history: List[CopilotMessage] = []
    
    async def query(
        self,
        user_message: str,
        conversation_id: Optional[str] = None,
    ) -> CopilotResponse:
        """
        Process a user query through the agent.
        
        Args:
            user_message: The user's question or statement
            conversation_id: Optional ID for tracking conversation
            
        Returns:
            CopilotResponse with answer and sources
        """
        # Detect intent
        intent, filters = detect_intent(user_message)
        
        # Prepare messages for the model
        messages = [
            {"role": "user", "content": user_message}
        ]
        
        # Limit conversation history
        recent_history = self.conversation_history[-self.MAX_HISTORY_MESSAGES:]
        for msg in recent_history:
            messages.append({"role": msg.role, "content": msg.content})
        
        # Build tools for the model
        tools = self._build_tools()
        
        # Call the AI provider
        response = await self.provider.generate(
            messages=messages,
            system_prompt=SYSTEM_PROMPT,
            tools=tools,
            max_tokens=1500,
            temperature=0.7,
        )
        
        # Process tool calls if any
        tool_results = []
        sources = []
        
        for _ in range(self.MAX_TOOL_CALLS):
            # In a real implementation, we would parse the model's tool calls
            # For now, execute based on detected intent
            break
        
        # Execute appropriate tool based on detected intent
        if intent == "GET_RECENT_CHANGES":
            result = await self.tools.get_recent_changes(
                time_window_hours=filters.get("time_window_hours", 24),
                minimum_severity=filters.get("minimum_severity", "SIGNIFICANT"),
            )
            tool_results.append(result)
            sources.extend(self._extract_sources(result))
        
        elif intent == "SEARCH_MARKETS":
            # Parse the user message for search terms
            search_query = user_message.replace("find markets", "").replace("search markets", "").strip()
            if not search_query:
                search_query = user_message
            result = await self.tools.search_markets(query=search_query, limit=10)
            tool_results.append(result)
            sources.extend(self._extract_sources(result))
        
        elif intent == "GET_SIGNALS":
            result = await self.tools.get_signals(
                limit=filters.get("limit", 50),
                severity=filters.get("severity"),
            )
            tool_results.append(result)
            sources.extend(self._extract_sources(result))
        
        elif intent == "COMPARE_MARKETS":
            # Extract market IDs from user message
            result = await self.tools.compare_markets(market_ids=[])
            tool_results.append(result)
            sources.extend(self._extract_sources(result))
        
        elif intent == "GET_MARKET_INTELLIGENCE":
            # Extract market ID from user message
            result = await self.tools.get_market_intelligence(market_id="")
            tool_results.append(result)
            sources.extend(self._extract_sources(result))
        
        # Update conversation history
        self.conversation_history.append(
            CopilotMessage(role="user", content=user_message, timestamp=datetime.now())
        )
        self.conversation_history.append(
            CopilotMessage(role="assistant", content=response.content, timestamp=datetime.now())
        )
        
        # Clean up conversation history
        if len(self.conversation_history) > self.MAX_HISTORY_MESSAGES * 2:
            self.conversation_history = self.conversation_history[-self.MAX_HISTORY_MESSAGES * 2:]
        
        return CopilotResponse(
            answer=response.content,
            sources=sources,
            intent=intent,
            tool_calls=[{"name": "execute_tool", "intent": intent} for _ in tool_results],
            generated_at=datetime.now(),
        )
    
    def _build_tools(self) -> List[Dict[str, Any]]:
        """Build the tools schema for the AI model."""
        return [
            {
                "type": "function",
                "function": {
                    "name": "search_markets",
                    "description": "Search for prediction markets by query",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "query": {"type": "string", "description": "Search query"},
                            "limit": {"type": "integer", "description": "Maximum results"},
                        },
                        "required": ["query"],
                    },
                },
            },
            {
                "type": "function",
                "function": {
                    "name": "get_market_intelligence",
                    "description": "Get detailed intelligence for a specific market",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "market_id": {"type": "string", "description": "Market identifier"},
                        },
                        "required": ["market_id"],
                    },
                },
            },
            {
                "type": "function",
                "function": {
                    "name": "get_signals",
                    "description": "Get signals for markets",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "market_id": {"type": "string", "description": "Optional market ID"},
                            "severity": {"type": "string", "description": "Minimum severity level"},
                            "limit": {"type": "integer", "description": "Maximum results"},
                        },
                    },
                },
            },
            {
                "type": "function",
                "function": {
                    "name": "get_recent_changes",
                    "description": "Get recent significant market changes",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "time_window_hours": {"type": "integer", "description": "Time window in hours"},
                            "minimum_severity": {"type": "string", "description": "Minimum severity"},
                        },
                    },
                },
            },
        ]
    
    def _extract_sources(self, result: ToolResult) -> List[Dict[str, Any]]:
        """Extract source references from tool results."""
        sources = []
        
        if result.data:
            if "markets" in result.data:
                for market in result.data.get("markets", []):
                    sources.append({
                        "type": "market",
                        "id": market.get("id"),
                        "title": market.get("title"),
                        "source": "prophet_database",
                    })
            
            if "signals" in result.data:
                for signal in result.data.get("signals", []):
                    sources.append({
                        "type": "signal",
                        "id": signal.get("id"),
                        "market_id": signal.get("market_id"),
                        "source": "prophet_database",
                    })
        
        return sources
    
    async def close(self):
        """Clean up resources."""
        await self.provider.close()


def create_agent(provider_name: str = "openai") -> CopilotAgent:
    """
    Create a Copilot agent with the specified provider.
    
    Args:
        provider_name: Name of the AI provider (openai, openrouter)
        
    Returns:
        CopilotAgent instance
    """
    # Get configuration from environment
    ai_api_key = os.getenv("AI_API_KEY", "")
    ai_model = os.getenv("AI_MODEL", "gpt-4o-mini")
    ai_base_url = os.getenv("AI_BASE_URL", "")
    postgres_url = os.getenv("DATABASE_URL", "")
    
    # Create provider
    if provider_name in ("openai", "openrouter"):
        provider = OpenAIProvider(model=ai_model, api_key=ai_api_key, base_url=ai_base_url)
    else:
        raise ValueError(f"Unknown provider: {provider_name}")
    
    # Create tools
    tools = AgentTools(postgres_url=postgres_url)
    
    # Create agent
    return CopilotAgent(provider=provider, tools=tools)
