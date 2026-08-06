"""Approximate token pricing, used only to populate the trace `cost` field.

These are ballpark public prices (USD per token) for cost-awareness in the
trace, not billing. Update if the provider changes pricing. Unknown models fall
back to zero, which leaves cost out of the trace.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Price:
    input_per_token: float
    output_per_token: float


# USD per token. gemini-2.0-flash approx: $0.10 / 1M input, $0.40 / 1M output.
_PRICES: dict[str, Price] = {
    "gemini-2.0-flash": Price(0.10 / 1_000_000, 0.40 / 1_000_000),
    "gemini-2.5-flash": Price(0.30 / 1_000_000, 2.50 / 1_000_000),
}


def estimate_cost(model: str, input_tokens: int, output_tokens: int) -> float:
    price = _PRICES.get(model)
    if price is None:
        return 0.0
    return input_tokens * price.input_per_token + output_tokens * price.output_per_token
