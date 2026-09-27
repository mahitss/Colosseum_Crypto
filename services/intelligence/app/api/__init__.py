"""API endpoints for Prophet Intelligence Service."""

from fastapi import APIRouter

from app.api.copilot import router as copilot_router
from app.api.market_studio import router as market_studio_router

router = APIRouter(prefix="/v1")

# Include all API routers
router.include_router(copilot_router)
router.include_router(market_studio_router)
