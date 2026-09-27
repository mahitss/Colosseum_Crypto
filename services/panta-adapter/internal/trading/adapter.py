"""Panta trading adapter for primary buy transactions."""

import os
from typing import Any, Dict, Optional
from decimal import Decimal
from datetime import datetime

import httpx
from pydantic import BaseModel, Field


class DecimalAmount(BaseModel):
    """Money type for trading amounts - never float64."""
    usdc_decimal: str  # Human-readable decimal string (e.g., "10.50")
    
    def __init__(self, amount: str):
        """Initialize with decimal string."""
        super().__init__(usdc_decimal=amount)
        # Validate it's a valid decimal
        try:
            Decimal(amount)
        except:
            raise ValueError(f"Invalid decimal amount: {amount}")


class BuyQuote(BaseModel):
    """Quote response from Panta."""
    market_id: str
    side: str  # YES or NO
    amount: DecimalAmount
    price_per_share: str
    total_cost: str  # In USDC
    shares_received: str
    slippage: Optional[str] = None
    quote_reference: str  # Unique reference for building
    expires_at: Optional[str] = None


class BuildTransactionResponse(BaseModel):
    """Unsigned transaction from Panta."""
    transaction_data: str  # Base64 or exact Panta format
    instructions: Optional[list] = None
    expected_wallet: str  # Public key that should sign
    expected_network: str  # Solana cluster (mainnet-beta, devnet, etc.)
    build_reference: str
    message_format: str  # e.g., "v0", "legacy"


class PantaTradingAdapter:
    """Adapter for Panta primary buy transactions."""
    
    def __init__(self, api_key: str, base_url: str = "https://live-api.panta.market/api/v1/"):
        """
        Initialize Panta trading adapter.
        
        Args:
            api_key: Panta API key from environment
            base_url: Panta API base URL (must end with /)
        """
        self.api_key = api_key
        self.base_url = base_url
        self.client = httpx.AsyncClient(
            base_url=base_url,
            headers={"X-Api-Key": api_key},
            timeout=30.0,
        )
    
    async def quote_primary_buy(
        self,
        market_id: str,
        side: str,  # "YES" or "NO"
        amount_usdc: str,  # Decimal string
        wallet_pubkey: str,
    ) -> BuyQuote:
        """
        Get a quote for a primary buy transaction.
        
        Args:
            market_id: Market identifier from Panta
            side: "YES" or "NO"
            amount_usdc: Amount in USDC as decimal string
            wallet_pubkey: User's Solana public key
            
        Returns:
            BuyQuote with pricing and quote reference
            
        Raises:
            httpx.HTTPError on API failure
        """
        # Validate inputs
        if side not in ("YES", "NO"):
            raise ValueError(f"Invalid side: {side}")
        
        try:
            Decimal(amount_usdc)
        except:
            raise ValueError(f"Invalid amount: {amount_usdc}")
        
        # Call Panta quote endpoint
        response = await self.client.post(
            "markets/primary-buy/quote/",  # Trailing slash required
            json={
                "market_id": market_id,
                "side": side,
                "amount_usdc": amount_usdc,  # Decimal string as per Panta docs
                "wallet": wallet_pubkey,
            },
        )
        response.raise_for_status()
        
        data = response.json()
        
        return BuyQuote(
            market_id=market_id,
            side=side,
            amount=DecimalAmount(amount_usdc),
            price_per_share=data.get("price_per_share", ""),
            total_cost=data.get("total_cost", ""),
            shares_received=data.get("shares_received", ""),
            slippage=data.get("slippage"),
            quote_reference=data.get("quote_reference", ""),
            expires_at=data.get("expires_at"),
        )
    
    async def build_primary_buy(
        self,
        quote_reference: str,
        wallet_pubkey: str,
    ) -> BuildTransactionResponse:
        """
        Build an unsigned transaction for the quoted trade.
        
        Args:
            quote_reference: Reference from quote step
            wallet_pubkey: User's Solana public key
            
        Returns:
            Unsigned transaction data
            
        Raises:
            httpx.HTTPError on API failure
        """
        response = await self.client.post(
            "markets/primary-buy/build/",  # Trailing slash required
            json={
                "quote_reference": quote_reference,
                "wallet": wallet_pubkey,
            },
        )
        response.raise_for_status()
        
        data = response.json()
        
        return BuildTransactionResponse(
            transaction_data=data.get("transaction", ""),
            instructions=data.get("instructions"),
            expected_wallet=data.get("expected_wallet", wallet_pubkey),
            expected_network=data.get("network", "mainnet-beta"),
            build_reference=data.get("build_reference", ""),
            message_format=data.get("message_format", "v0"),
        )
    
    async def report_transaction(
        self,
        market_id: str,
        side: str,
        amount_usdc: str,
        signature: str,
        quote_reference: str,
    ) -> Dict[str, Any]:
        """
        Report a signed and broadcast transaction to Panta.
        
        Args:
            market_id: Market identifier
            side: YES or NO
            amount_usdc: Amount traded
            signature: Solana transaction signature
            quote_reference: Original quote reference
            
        Returns:
            Response from Panta with order confirmation
            
        Raises:
            httpx.HTTPError on API failure
        """
        response = await self.client.post(
            "markets/primary-buy/report/",  # Trailing slash required
            json={
                "market_id": market_id,
                "side": side,
                "amount_usdc": amount_usdc,
                "signature": signature,
                "quote_reference": quote_reference,
            },
        )
        response.raise_for_status()
        
        return response.json()
    
    async def verify_transaction(
        self,
        signature: str,
    ) -> Dict[str, Any]:
        """
        Verify transaction status with Panta.
        
        Args:
            signature: Solana transaction signature
            
        Returns:
            Verification status from Panta
            
        Raises:
            httpx.HTTPError on API failure
        """
        response = await self.client.get(
            f"transactions/{signature}/verify/",  # Trailing slash required
        )
        response.raise_for_status()
        
        return response.json()
    
    async def close(self):
        """Close HTTP client."""
        await self.client.aclose()


def get_trading_adapter() -> PantaTradingAdapter:
    """Get configured Panta trading adapter."""
    api_key = os.getenv("PANTA_API_KEY", "")
    if not api_key:
        raise ValueError("PANTA_API_KEY environment variable is required")
    
    return PantaTradingAdapter(api_key=api_key)
