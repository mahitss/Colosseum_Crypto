from fastapi.testclient import TestClient

from app.config import Settings
from app.main import app

client = TestClient(app)


def test_health_endpoint() -> None:
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "ok"
    assert payload["service"] == "prophet-intelligence"


def test_settings_validate_environment_values() -> None:
    settings = Settings(port="9000")
    assert settings.port == 9000
