"""System prompt and intent routing for Prophet Copilot."""

# System prompt for the AI agent
SYSTEM_PROMPT = """You are Prophet, a prediction-market intelligence assistant.

You analyze prediction-market information sourced from Prophet and Panta.

You report observed market data.

You distinguish:
- observation
- calculation
- interpretation
- uncertainty

You do not present market movement as certainty.

You do not provide financial advice.

You never invent data.

When data is insufficient, explicitly say so.

You must use available tools before making claims about current market conditions.

Never claim to have accessed information that tools did not return.

Your responses should be concise and factual. Cite your sources.

If asked about future predictions, clarify that you report observed market probabilities, not predictions.

Do not use SQL or code generation. Use the available tools to retrieve data.

When a tool returns data, use it directly. Do not fabricate numbers.

If a request cannot be fulfilled with available tools, explain why.

Example of good response:
"Three significant probability shifts were detected in the last 24 hours.

1. Market X moved from 42% to 58% (+16 percentage points).
2. Market Y moved from 61% to 52% (-9 percentage points).
3. Market Z moved from 31% to 40% (+9 percentage points).

These are observed market movements, not predictions or recommendations.

[Market: BTC > $120K]"""

# Intent types
INTENT_TYPES = [
    "SEARCH_MARKETS",
    "GET_MARKET",
    "GET_MARKET_INTELLIGENCE",
    "GET_SIGNALS",
    "COMPARE_MARKETS",
    "GET_RECENT_CHANGES",
    "GENERAL_QUERY",
    "UNRECOGNIZED",
]

# Known market identifiers (for intent routing)
MARKET_IDENTIFIERS = [
    "bitcoin", "btc", "ethereum", "eth", "solana", "sol",
    "prediction market", "panta",
]


def detect_intent(user_message: str) -> tuple[str, dict]:
    """
    Detect the user's intent and extract filters.
    
    Returns:
        tuple: (intent, filters)
    """
    message_lower = user_message.lower()
    
    # Check for specific intent patterns
    if any(phrase in message_lower for phrase in ["changed significantly", "what changed", "recent changes", "significant shifts"]):
        return "GET_RECENT_CHANGES", {"time_window_hours": 24, "minimum_severity": "SIGNIFICANT"}
    
    if any(phrase in message_lower for phrase in ["find markets", "search markets", "markets with", "markets below"]):
        filters = {}
        # Extract probability filter if present
        if "below" in message_lower or "under" in message_lower:
            for word in message_lower.split():
                cleaned = word.rstrip("%")
                if cleaned.replace(".", "", 1).isdigit():
                    filters["probability_max"] = float(cleaned) / 100
                    break
        return "SEARCH_MARKETS", filters
    
    if any(phrase in message_lower for phrase in ["compare", "vs", "versus"]):
        return "COMPARE_MARKETS", {}
    
    if any(phrase in message_lower for phrase in ["tell me about", "what is", "explain"]):
        # Try to extract market name/ID
        return "GET_MARKET_INTELLIGENCE", {}
    
    if any(phrase in message_lower for phrase in ["signals", "signal", "biggest"]):
        return "GET_SIGNALS", {}
    
    if any(phrase in message_lower for phrase in ["most", "highest", "top"]):
        return "GET_RECENT_CHANGES", {"time_window_hours": 24, "minimum_severity": "WATCH"}
    
    return "GENERAL_QUERY", {}
