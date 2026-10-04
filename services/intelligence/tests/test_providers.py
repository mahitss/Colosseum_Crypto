"""Tests for AI Provider implementations using mocked HTTP clients."""

import os
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from app.providers.base import AIProvider, AIResponse, ToolCall
from app.providers.openai import OpenAIProvider


class MockOpenAIClient:
    """Mock OpenAI client for testing without real API calls."""
    
    def __init__(self):
        self.chat = MagicMock()
        self.chat.completions = MagicMock()
        self.chat.completions.create = AsyncMock()
        self.close = AsyncMock()


@pytest.fixture
def mock_openai_client():
    """Create a mock OpenAI client."""
    return MockOpenAIClient()


@pytest.fixture
def provider_config():
    """Basic provider configuration."""
    return {
        "model": "test-model",
        "api_key": "test-key",
        "base_url": "https://test.openrouter.ai/api/v1",
    }


class TestOpenAIProvider:
    """Tests for OpenAI/OpenRouter provider."""
    
    def test_provider_initialization_with_base_url(self, provider_config):
        """Test provider initializes with custom base URL."""
        with patch("app.providers.openai.openai") as mock_openai_module:
            mock_client = MagicMock()
            mock_openai_module.AsyncOpenAI.return_value = mock_client
            
            provider = OpenAIProvider(
                model=provider_config["model"],
                api_key=provider_config["api_key"],
                base_url=provider_config["base_url"],
            )
            
            # Verify AsyncOpenAI was called with base_url
            mock_openai_module.AsyncOpenAI.assert_called_once_with(
                api_key=provider_config["api_key"],
                base_url=provider_config["base_url"],
            )
            assert provider.model == provider_config["model"]
            assert provider.api_key == provider_config["api_key"]
    
    def test_provider_initialization_without_base_url(self, provider_config):
        """Test provider initializes without base URL (defaults to OpenAI)."""
        with patch("app.providers.openai.openai") as mock_openai_module:
            mock_client = MagicMock()
            mock_openai_module.AsyncOpenAI.return_value = mock_client
            
            provider = OpenAIProvider(
                model=provider_config["model"],
                api_key=provider_config["api_key"],
                base_url="",
            )
            
            # Verify AsyncOpenAI was called with base_url=None
            mock_openai_module.AsyncOpenAI.assert_called_once_with(
                api_key=provider_config["api_key"],
                base_url=None,
            )
    
    @pytest.mark.asyncio
    async def test_generate_success(self, provider_config, mock_openai_client):
        """Test successful generation response."""
        with patch("app.providers.openai.openai") as mock_openai_module:
            mock_openai_module.AsyncOpenAI.return_value = mock_openai_client
            
            # Setup mock response
            mock_response = MagicMock()
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.content = "Test response"
            mock_response.choices[0].message.tool_calls = None
            mock_response.usage = MagicMock()
            mock_response.usage.prompt_tokens = 10
            mock_response.usage.completion_tokens = 20
            
            mock_openai_client.chat.completions.create.return_value = mock_response
            
            provider = OpenAIProvider(
                model=provider_config["model"],
                api_key=provider_config["api_key"],
                base_url=provider_config["base_url"],
            )
            
            response = await provider.generate(
                messages=[{"role": "user", "content": "Hello"}],
                system_prompt="You are a test assistant",
                max_tokens=100,
                temperature=0.5,
            )
            
            assert isinstance(response, AIResponse)
            assert response.content == "Test response"
            assert response.model == provider_config["model"]
            assert response.provider == "openai"
            assert response.usage["prompt_tokens"] == 10
            assert response.usage["completion_tokens"] == 20
    
    @patch("app.providers.openai.openai")
    @pytest.mark.asyncio
    async def test_generate_with_tool_calls(self, mock_openai_module, provider_config, mock_openai_client):
        """Test generation with tool calls."""
        mock_openai_module.AsyncOpenAI.return_value = mock_openai_client
        
        # Setup mock response with tool calls
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "I'll search for that"
        mock_tool_call = MagicMock()
        mock_tool_call.function.name = "search_markets"
        mock_tool_call.function.arguments = '{"query": "bitcoin", "limit": 10}'
        mock_response.choices[0].message.tool_calls = [mock_tool_call]
        mock_response.usage = MagicMock()
        mock_response.usage.prompt_tokens = 15
        mock_response.usage.completion_tokens = 25
        
        mock_openai_client.chat.completions.create.return_value = mock_response
        
        provider = OpenAIProvider(
            model=provider_config["model"],
            api_key=provider_config["api_key"],
            base_url=provider_config["base_url"],
        )
        
        response = await provider.generate(
            messages=[{"role": "user", "content": "Search for bitcoin markets"}],
            system_prompt="You are a test assistant",
            tools=[{"type": "function", "function": {"name": "search_markets"}}],
        )
        
        assert isinstance(response, AIResponse)
        assert response.content == "I'll search for that"
        assert len(response.tool_calls) == 1
        assert response.tool_calls[0].name == "search_markets"
        assert response.tool_calls[0].arguments == {"query": "bitcoin", "limit": 10}
    
    @pytest.mark.asyncio
    async def test_generate_structured_success(self, provider_config, mock_openai_client):
        """Test structured generation response."""
        with patch("app.providers.openai.openai") as mock_openai_module:
            mock_openai_module.AsyncOpenAI.return_value = mock_openai_client
            
            # Setup mock response
            mock_response = MagicMock()
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.content = '{"answer": "42"}'
            
            mock_openai_client.chat.completions.create.return_value = mock_response
            
            from pydantic import BaseModel
            
            class TestResponse(BaseModel):
                answer: str
            
            provider = OpenAIProvider(
                model=provider_config["model"],
                api_key=provider_config["api_key"],
                base_url=provider_config["base_url"],
            )
            
            response = await provider.generate_structured(
                messages=[{"role": "user", "content": "What is the answer?"}],
                system_prompt="You are a test assistant",
                response_model=TestResponse,
            )
            
            assert isinstance(response, AIResponse)
            assert response.content == '{"answer": "42"}'
            assert response.model == provider_config["model"]
            assert response.provider == "openai"
    
    @pytest.mark.asyncio
    async def test_close(self, provider_config, mock_openai_client):
        """Test provider close method."""
        with patch("app.providers.openai.openai") as mock_openai_module:
            mock_openai_module.AsyncOpenAI.return_value = mock_openai_client
            
            provider = OpenAIProvider(
                model=provider_config["model"],
                api_key=provider_config["api_key"],
                base_url=provider_config["base_url"],
            )
            
            await provider.close()
            
            mock_openai_client.close.assert_called_once()


class TestCreateAgent:
    """Tests for agent creation with different providers."""
    
    def test_create_agent_openai(self):
        """Test agent creation with OpenAI provider."""
        with patch("app.agent.agent.OpenAIProvider") as mock_provider_class:
            mock_provider = MagicMock()
            mock_provider_class.return_value = mock_provider
            
            from app.agent.agent import create_agent
            
            with patch.dict("os.environ", {
                "AI_API_KEY": "test-key",
                "AI_MODEL": "gpt-4o-mini",
                "AI_BASE_URL": "",
                "DATABASE_URL": "postgresql://test",
            }):
                agent = create_agent("openai")
                
                assert agent is not None
                mock_provider_class.assert_called_once_with(
                    model="gpt-4o-mini",
                    api_key="test-key",
                    base_url="",
                )
    
    def test_create_agent_openrouter(self):
        """Test agent creation with OpenRouter provider."""
        with patch("app.agent.agent.OpenAIProvider") as mock_provider_class:
            mock_provider = MagicMock()
            mock_provider_class.return_value = mock_provider
            
            from app.agent.agent import create_agent
            
            with patch.dict("os.environ", {
                "AI_API_KEY": "test-key",
                "AI_MODEL": "apodex/apodex-1.1-mini:free",
                "AI_BASE_URL": "https://openrouter.ai/api/v1",
                "DATABASE_URL": "postgresql://test",
            }):
                agent = create_agent("openrouter")
                
                assert agent is not None
                mock_provider_class.assert_called_once_with(
                    model="apodex/apodex-1.1-mini:free",
                    api_key="test-key",
                    base_url="https://openrouter.ai/api/v1",
                )
    
    def test_create_agent_unknown_provider(self):
        """Test agent creation with unknown provider raises error."""
        from app.agent.agent import create_agent
        
        with patch.dict("os.environ", {
            "AI_API_KEY": "test-key",
            "AI_MODEL": "gpt-4o-mini",
            "DATABASE_URL": "postgresql://test",
        }):
            with pytest.raises(ValueError, match="Unknown provider: unknown"):
                create_agent("unknown")


class TestConfigSettings:
    """Tests for configuration settings."""
    
    def test_settings_with_openrouter_env(self):
        """Test settings load OpenRouter configuration from environment."""
        from app.config import Settings
        
        with patch.dict("os.environ", {
            "AI_PROVIDER": "openrouter",
            "AI_BASE_URL": "https://openrouter.ai/api/v1",
            "AI_API_KEY": "sk-or-test-key",
            "AI_MODEL": "apodex/apodex-1.1-mini:free",
        }):
            settings = Settings(
                ai_provider=os.getenv("AI_PROVIDER", "openai"),
                ai_api_key=os.getenv("AI_API_KEY", ""),
                ai_base_url=os.getenv("AI_BASE_URL", ""),
                ai_model=os.getenv("AI_MODEL", "gpt-4o-mini"),
            )
            
            assert settings.ai_provider == "openrouter"
            assert settings.ai_base_url == "https://openrouter.ai/api/v1"
            assert settings.ai_api_key == "sk-or-test-key"
            assert settings.ai_model == "apodex/apodex-1.1-mini:free"
    
    def test_settings_defaults(self):
        """Test settings defaults when env vars not set."""
        from app.config import Settings
        
        with patch.dict("os.environ", {}, clear=True):
            settings = Settings(
                ai_provider=os.getenv("AI_PROVIDER", "openai"),
                ai_api_key=os.getenv("AI_API_KEY", ""),
                ai_base_url=os.getenv("AI_BASE_URL", ""),
                ai_model=os.getenv("AI_MODEL", "gpt-4o-mini"),
            )
            
            assert settings.ai_provider == "openai"
            assert settings.ai_base_url == ""
            assert settings.ai_api_key == ""
            assert settings.ai_model == "gpt-4o-mini"