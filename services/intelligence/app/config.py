import os

from pydantic import BaseModel, Field


class Settings(BaseModel):
    app_name: str = "prophet-intelligence"
    environment: str = "development"
    port: int = Field(default=8001, ge=1, le=65535)
    ai_provider: str = "openai"
    ai_api_key: str = ""
    cors_allowed_origins: list[str] = Field(default_factory=list)


settings = Settings(
    environment=os.getenv("APP_ENV", "development"),
    port=os.getenv("PORT", "8001"),
    ai_provider=os.getenv("AI_PROVIDER", "openai"),
    ai_api_key=os.getenv("AI_API_KEY", ""),
    cors_allowed_origins=os.getenv("CORS_ALLOWED_ORIGINS", "").split(",") if os.getenv("CORS_ALLOWED_ORIGINS") else [],
)
