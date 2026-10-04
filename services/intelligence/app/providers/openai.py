"""OpenAI/OpenRouter provider implementation for Prophet Copilot."""

import os
from typing import Any, Dict, List, Optional
from pydantic import BaseModel

from app.providers.base import AIProvider, AIResponse, ToolCall


try:
    import openai
except ImportError:
    openai = None


class OpenAIProvider(AIProvider):
    """OpenAI/OpenRouter provider implementation. OpenRouter uses OpenAI-compatible API."""
    
    def __init__(self, model: str, api_key: str, base_url: str = ""):
        if not openai:
            raise ImportError("openai package is required. Install with: pip install openai")
        super().__init__(model, api_key)
        self.client = openai.AsyncOpenAI(
            api_key=api_key,
            base_url=base_url if base_url else None,
        )
    
    async def generate(
        self,
        messages: List[Dict[str, str]],
        system_prompt: str,
        tools: Optional[List[Dict[str, Any]]] = None,
        max_tokens: int = 1000,
        temperature: float = 0.7,
    ) -> AIResponse:
        """Generate a response from OpenAI."""
        messages_with_system = [{"role": "system", "content": system_prompt}]
        messages_with_system.extend(messages)
        
        params = {
            "model": self.model,
            "messages": messages_with_system,
            "max_tokens": max_tokens,
            "temperature": temperature,
        }
        
        if tools:
            params["tools"] = tools
        
        response = await self.client.chat.completions.create(**params)
        
        content = response.choices[0].message.content or ""
        
        # Extract tool calls if present
        tool_calls = []
        if response.choices[0].message.tool_calls:
            for tc in response.choices[0].message.tool_calls:
                import json
                try:
                    arguments = json.loads(tc.function.arguments)
                except (json.JSONDecodeError, TypeError):
                    arguments = {}
                tool_calls.append(ToolCall(
                    name=tc.function.name,
                    arguments=arguments,
                ))
        
        return AIResponse(
            content=content,
            model=self.model,
            provider="openai",
            usage={
                "prompt_tokens": response.usage.prompt_tokens if response.usage else 0,
                "completion_tokens": response.usage.completion_tokens if response.usage else 0,
            },
            tool_calls=tool_calls if tool_calls else None,
        )
    
    async def generate_structured(
        self,
        messages: List[Dict[str, str]],
        system_prompt: str,
        response_model: type,
        tools: Optional[List[Dict[str, Any]]] = None,
    ) -> AIResponse:
        """Generate a structured response from OpenAI using function calling."""
        # Convert response_model to a JSON schema for function calling
        schema = response_model.model_json_schema()
        
        # Create a tool that matches the schema
        tool = {
            "type": "function",
            "function": {
                "name": "generate_response",
                "description": schema.get("description", "Generate a structured response"),
                "parameters": schema,
            },
        }
        
        messages_with_system = [{"role": "system", "content": system_prompt}]
        messages_with_system.extend(messages)
        
        response = await self.client.chat.completions.create(
            model=self.model,
            messages=messages_with_system,
            tools=[tool] if tools else None,
            tool_choice="required" if tools else None,
        )
        
        content = response.choices[0].message.content or ""
        return AIResponse(
            content=content,
            model=self.model,
            provider="openai",
        )
    
    async def close(self):
        """Close the OpenAI client."""
        await self.client.close()
