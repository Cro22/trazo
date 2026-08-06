"""Builds the Gemini chat model. Kept tiny so it is easy to swap for a fake."""

from __future__ import annotations

import os
from typing import Optional

from langchain_core.language_models import BaseChatModel

# gemini-2.0-flash was retired by the API (404 NOT_FOUND); 2.5-flash is the
# current cheap flash tier. Override with --model if you want another.
DEFAULT_MODEL = "gemini-2.5-flash"


class MissingAPIKey(RuntimeError):
    pass


def build_llm(
    model: str = DEFAULT_MODEL,
    *,
    api_key: Optional[str] = None,
    temperature: float = 0.0,
    max_output_tokens: int = 512,
) -> BaseChatModel:
    """Build a ChatGoogleGenerativeAI. The key is read from GEMINI_API_KEY unless
    passed explicitly. Cost discipline: temperature 0 and a low token cap."""
    key = api_key or os.environ.get("GEMINI_API_KEY")
    if not key:
        raise MissingAPIKey(
            "GEMINI_API_KEY is not set. Export it or pass --api-key to run against Gemini."
        )
    # Imported here so tests that use a fake model never import the provider.
    from langchain_google_genai import ChatGoogleGenerativeAI

    return ChatGoogleGenerativeAI(
        model=model,
        google_api_key=key,
        temperature=temperature,
        max_output_tokens=max_output_tokens,
    )
