from fastapi import FastAPI
from pydantic import BaseModel

from app.config import settings

class HealthResponse(BaseModel):
    status: str
    service: str


app = FastAPI(title=settings.app_name)


@app.get("/health")
def health() -> HealthResponse:
    return {"status": "ok", "service": settings.app_name}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app.main:app", host="0.0.0.0", port=settings.port, reload=False)
