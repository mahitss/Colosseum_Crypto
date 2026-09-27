"""Prophet Market Studio — AI-assisted market drafting.

The AI structures and assists. It never decides resolution rules without human
review, never signs, never broadcasts, and never creates a market on its own.

The schemas in this package mirror the *verified* Panta create API only. Fields
that Panta does not support (for example per-outcome YES/NO text) are modelled
as Prophet-side presentation data and are explicitly marked as such, so that we
never send an invented field upstream.
"""
