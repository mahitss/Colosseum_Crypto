import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    app_name: str = "prophet-intelligence"
    environment: str = os.getenv("APP_ENV", "development")
    port: int = int(os.getenv("PORT", "8001"))
    ai_provider: str = os.getenv("AI_PROVIDER", "openai")
    ai_api_key: str = os.getenv("AI_API_KEY", "")


settings = Settings()
