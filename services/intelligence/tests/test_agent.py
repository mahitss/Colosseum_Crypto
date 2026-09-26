"""Tests for Prophet AI Copilot agent."""

import pytest
from app.agent.prompts import detect_intent


def test_detect_intent_recent_changes():
    """Test detection of GET_RECENT_CHANGES intent."""
    intent, filters = detect_intent("What changed significantly today?")
    assert intent == "GET_RECENT_CHANGES"
    assert filters["time_window_hours"] == 24
    assert filters["minimum_severity"] == "SIGNIFICANT"


def test_detect_intent_search_markets():
    """Test detection of SEARCH_MARKETS intent."""
    intent, filters = detect_intent("Find markets about Bitcoin")
    assert intent == "SEARCH_MARKETS"


def test_detect_intent_search_markets_with_probability():
    """Test detection of SEARCH_MARKETS with probability filter."""
    intent, filters = detect_intent("Find markets below 40%")
    assert intent == "SEARCH_MARKETS"
    assert "probability_max" in filters
    assert filters["probability_max"] == 0.4


def test_detect_intent_compare_markets():
    """Test detection of COMPARE_MARKETS intent."""
    intent, filters = detect_intent("Compare market A vs market B")
    assert intent == "COMPARE_MARKETS"


def test_detect_intent_get_signals():
    """Test detection of GET_SIGNALS intent."""
    intent, filters = detect_intent("Show me the signals")
    assert intent == "GET_SIGNALS"


def test_detect_intent_general_query():
    """Test detection of GENERAL_QUERY for unknown patterns."""
    intent, filters = detect_intent("Hello, how are you?")
    assert intent == "GENERAL_QUERY"
    assert filters == {}
