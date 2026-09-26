"""Agent tools for Prophet Copilot - with real PostgreSQL queries."""

from typing import Any, Dict, List, Optional
from datetime import datetime, timedelta
import os

from pydantic import BaseModel

try:
    import asyncpg
    HAS_ASYNCPG = True
except ImportError:
    HAS_ASYNCPG = False


class Market(BaseModel):
    """Market model."""
    id: str
    source: str
    source_market_id: str
    title: str
    description: Optional[str] = None
    category: Optional[str] = None
    status: str
    yes_probability: Optional[str] = None
    no_probability: Optional[str] = None
    liquidity: Optional[str] = None
    volume_usdc: Optional[str] = None
    created_at: str
    closes_at: Optional[str] = None


class Signal(BaseModel):
    """Signal model."""
    id: int
    market_id: str
    signal_type: str
    severity: str
    previous_value: Optional[str] = None
    current_value: Optional[str] = None
    percentage_points: Optional[str] = None
    timestamp: str


class ToolResult(BaseModel):
    """Result from a tool call."""
    success: bool
    data: Optional[Any] = None
    error: Optional[str] = None
    sources: List[Dict[str, Any]] = []


class AgentTools:
    """Agent tools for Prophet Copilot with real PostgreSQL queries."""
    
    def __init__(self, postgres_url: str):
        self.postgres_url = postgres_url
        self._pool: Optional[Any] = None
    
    async def _get_pool(self):
        """Get or create database connection pool."""
        if not HAS_ASYNCPG:
            return None
        if self._pool is None:
            self._pool = await asyncpg.create_pool(
                self.postgres_url,
                min_size=2,
                max_size=10,
            )
        return self._pool
    
    def _row_to_dict(self, row: Any) -> Dict[str, Any]:
        """Convert asyncpg row to dict with proper datetime handling."""
        result = dict(row)
        for key, value in result.items():
            if hasattr(value, 'isoformat'):
                result[key] = value.isoformat()
        return result
    
    async def search_markets(self, query: str, limit: int = 10) -> ToolResult:
        """Search for markets by title or description."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        try:
            rows = await pool.fetch("""
                SELECT id, source, source_market_id, title, description, category, 
                       status, yes_probability, no_probability, liquidity, volume_usdc, 
                       created_at, closes_at, resolution_status
                FROM markets 
                WHERE LOWER(title) LIKE $1 OR LOWER(description) LIKE $1
                ORDER BY created_at DESC 
                LIMIT $2
            """, f"%{query.lower()}%", limit)
            
            markets = [self._row_to_dict(r) for r in rows]
            return ToolResult(success=True, data={"markets": markets, "total": len(markets)}, sources=markets)
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])
    
    async def get_market(self, market_id: str) -> ToolResult:
        """Get market details by ID."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        try:
            row = await pool.fetchrow("""
                SELECT id, source, source_market_id, title, description, category,
                       status, yes_probability, no_probability, liquidity, volume_usdc,
                       created_at, closes_at, resolution_status
                FROM markets WHERE id = $1
            """, market_id)
            
            if not row:
                return ToolResult(success=False, error="Market not found", data=None, sources=[])
            
            market = self._row_to_dict(row)
            return ToolResult(success=True, data={"market": market}, sources=[market])
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])
    
    async def get_market_intelligence(self, market_id: str) -> ToolResult:
        """Get market intelligence (latest observation, signals, metrics)."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        try:
            # Get market
            market_row = await pool.fetchrow("SELECT * FROM markets WHERE id = $1", market_id)
            market = self._row_to_dict(market_row) if market_row else None
            
            # Get latest observation
            obs_row = await pool.fetchrow("""
                SELECT * FROM market_observations 
                WHERE market_id = $1 
                ORDER BY observed_at DESC LIMIT 1
            """, market_id)
            latest_observation = self._row_to_dict(obs_row) if obs_row else None
            
            # Get recent signals
            signal_rows = await pool.fetch("""
                SELECT * FROM signals 
                WHERE market_id = $1 
                ORDER BY timestamp DESC LIMIT 10
            """, market_id)
            signals = [self._row_to_dict(r) for r in signal_rows]
            
            return ToolResult(
                success=True, 
                data={
                    "market": market,
                    "latest_observation": latest_observation,
                    "signals": signals,
                }, 
                sources=[market] if market else []
            )
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])
    
    async def get_signals(
        self,
        market_id: Optional[str] = None,
        signal_type: Optional[str] = None,
        severity: Optional[str] = None,
        time_window_hours: int = 24,
        limit: int = 50,
    ) -> ToolResult:
        """Get signals with optional filtering."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        try:
            conditions = ["timestamp >= now() - interval '1 hour' * $1"]
            params = [time_window_hours]
            param_idx = 2
            
            if market_id:
                conditions.append(f"market_id = ${param_idx}")
                params.append(market_id)
                param_idx += 1
            if signal_type:
                conditions.append(f"signal_type = ${param_idx}")
                params.append(signal_type)
                param_idx += 1
            if severity:
                conditions.append(f"severity = ${param_idx}")
                params.append(severity)
            
            where_clause = " AND ".join(conditions)
            params.append(limit)
            
            rows = await pool.fetch(f"""
                SELECT * FROM signals 
                WHERE {where_clause}
                ORDER BY timestamp DESC LIMIT ${param_idx}
            """, *params)
            
            signals = [self._row_to_dict(r) for r in rows]
            return ToolResult(success=True, data={"signals": signals, "total": len(signals)}, sources=signals)
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])
    
    async def compare_markets(self, market_ids: List[str]) -> ToolResult:
        """Compare multiple markets."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        if not market_ids:
            return ToolResult(success=False, error="No market IDs provided", data=None, sources=[])
        
        try:
            placeholders = ",".join([f"${i+1}" for i in range(len(market_ids))])
            rows = await pool.fetch(f"""
                SELECT id, source, source_market_id, title, yes_probability, 
                       no_probability, liquidity, volume_usdc, created_at
                FROM markets WHERE id IN ({placeholders})
            """, *market_ids)
            
            markets = [self._row_to_dict(r) for r in rows]
            return ToolResult(success=True, data={"markets": markets}, sources=markets)
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])
    
    async def get_recent_changes(
        self,
        time_window_hours: int = 24,
        minimum_severity: str = "SIGNIFICANT",
    ) -> ToolResult:
        """Get recent significant changes."""
        pool = await self._get_pool()
        if not pool:
            return ToolResult(success=False, error="Database not available", data=None, sources=[])
        
        try:
            rows = await pool.fetch("""
                SELECT s.*, m.title as market_title
                FROM signals s
                JOIN markets m ON s.market_id = m.id
                WHERE s.severity IN ('SIGNIFICANT', 'CRITICAL')
                  AND s.timestamp >= now() - interval '1 hour' * $1
                ORDER BY s.timestamp DESC
                LIMIT 50
            """, time_window_hours)
            
            changes = []
            for r in rows:
                change = self._row_to_dict(r)
                change['market_title'] = r.get('market_title')
                changes.append(change)
            
            return ToolResult(success=True, data={"changes": changes}, sources=changes)
        except Exception as e:
            return ToolResult(success=False, error=str(e), data=None, sources=[])


# Export tools class
__all__ = ["AgentTools", "ToolResult", "Market", "Signal"]
