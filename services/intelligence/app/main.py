from fastapi import FastAPI
from pydantic import BaseModel
from fastapi.middleware.cors import CORSMiddleware

from app.config import settings
from app.api import router as api_router

class HealthResponse(BaseModel):
    status: str
    service: str


app = FastAPI(title=settings.app_name)

# CORS middleware - restricted to configured origins in production
# In development, allow all origins for convenience
allowed_origins = ["*"] if settings.environment == "development" else settings.cors_allowed_origins
app.add_middleware(
    CORSMiddleware,
    allow_origins=allowed_origins,
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type", "X-Request-Id"],
)

# Health endpoint
@app.get("/health")
def health() -> HealthResponse:
    return {"status": "ok", "service": settings.app_name}

# API endpoints
app.include_router(api_router)

if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app.main:app", host="0.0.0.0", port=settings.port, reload=False)
