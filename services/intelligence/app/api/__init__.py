"""API endpoints for Prophet Intelligence Service."""

from fastapi import APIRouter

from app.api.copilot import router as copilot_router

router = APIRouter(prefix="/v1")

# Include all API routers
router.include_router(copilot_router)
