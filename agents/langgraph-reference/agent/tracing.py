"""Callback handler that turns LangChain/LangGraph events into trazo steps.

The agent code stays free of tracing: we attach this handler via the run config
and it records an llm_call per model turn, a tool_call per tool invocation and a
tool_result per return (or error). Steps are appended in event order, which is
chronological, so the emitted trace pairs correctly for ToolCallEvaluator.

Timestamps come from the recorder's own clock (so tests with an injected clock
stay deterministic); this handler only measures wall-clock duration.
"""

from __future__ import annotations

import json
import time
from typing import Any, Optional
from uuid import UUID

from langchain_core.callbacks import BaseCallbackHandler
from langchain_core.outputs import LLMResult

from .pricing import estimate_cost
from trazo_emitter import ToolCall, TraceRecorder


def _elapsed_ms(start_perf: float) -> int:
    return int((time.perf_counter() - start_perf) * 1000)


def _jsonable(value: Any) -> Any:
    """Coerce a tool output into something JSON serializable for the trace."""
    if hasattr(value, "content"):  # e.g. a ToolMessage
        value = value.content
    if isinstance(value, (dict, list, int, float, bool)) or value is None:
        return value
    text = str(value)
    try:
        return json.loads(text)
    except (json.JSONDecodeError, ValueError):
        return text


class TracingCallbackHandler(BaseCallbackHandler):
    def __init__(self, recorder: TraceRecorder, *, model: str) -> None:
        self._rec = recorder
        self._model = model
        self._llm_start: dict[UUID, float] = {}
        self._tool_start: dict[UUID, float] = {}
        self._pending: dict[UUID, ToolCall] = {}

    # -- LLM ----------------------------------------------------------------

    def on_chat_model_start(
        self, serialized: dict, messages: list, *, run_id: UUID, **kwargs: Any
    ) -> None:
        self._llm_start[run_id] = time.perf_counter()

    def on_llm_start(
        self, serialized: dict, prompts: list, *, run_id: UUID, **kwargs: Any
    ) -> None:
        self._llm_start.setdefault(run_id, time.perf_counter())

    def on_llm_end(self, response: LLMResult, *, run_id: UUID, **kwargs: Any) -> None:
        started = self._llm_start.pop(run_id, None)
        duration_ms = _elapsed_ms(started) if started is not None else 0

        text, in_tok, out_tok = _read_generation(response)
        cost = estimate_cost(self._model, in_tok, out_tok)
        self._rec.record_llm_call(
            self._model,
            output=text,
            input_tokens=in_tok,
            output_tokens=out_tok,
            cost=cost,
            duration_ms=duration_ms,
        )

    # -- Tools --------------------------------------------------------------

    def on_tool_start(
        self,
        serialized: dict,
        input_str: str,
        *,
        run_id: UUID,
        inputs: Optional[dict] = None,
        **kwargs: Any,
    ) -> None:
        name = _tool_name(serialized, kwargs)
        tool_input = inputs if inputs is not None else _maybe_json(input_str)
        self._tool_start[run_id] = time.perf_counter()
        self._pending[run_id] = self._rec.record_tool_call(name, input=tool_input)

    def on_tool_end(self, output: Any, *, run_id: UUID, **kwargs: Any) -> None:
        call = self._pending.pop(run_id, None)
        started = self._tool_start.pop(run_id, None)
        duration_ms = _elapsed_ms(started) if started is not None else 0
        self._rec.record_tool_result(
            call if call is not None else "tool",
            output=_jsonable(output),
            duration_ms=duration_ms,
        )

    def on_tool_error(self, error: BaseException, *, run_id: UUID, **kwargs: Any) -> None:
        call = self._pending.pop(run_id, None)
        started = self._tool_start.pop(run_id, None)
        duration_ms = _elapsed_ms(started) if started is not None else 0
        self._rec.record_tool_result(
            call if call is not None else "tool",
            error=str(error),
            duration_ms=duration_ms,
        )


def _tool_name(serialized: Optional[dict], kwargs: dict) -> str:
    if isinstance(serialized, dict) and serialized.get("name"):
        return serialized["name"]
    return kwargs.get("name") or "tool"


def _maybe_json(text: str) -> Any:
    try:
        return json.loads(text)
    except (json.JSONDecodeError, ValueError, TypeError):
        return text


def _read_generation(response: LLMResult) -> tuple[str, int, int]:
    """Extract (text, input_tokens, output_tokens) from an LLMResult."""
    text = ""
    in_tok = 0
    out_tok = 0
    if response.generations and response.generations[0]:
        gen = response.generations[0][0]
        text = gen.text or ""
        message = getattr(gen, "message", None)
        usage = getattr(message, "usage_metadata", None)
        if usage:
            in_tok = int(usage.get("input_tokens", 0) or 0)
            out_tok = int(usage.get("output_tokens", 0) or 0)
        if not text and message is not None:
            tool_calls = getattr(message, "tool_calls", None)
            if tool_calls:
                names = ", ".join(tc.get("name", "?") for tc in tool_calls)
                text = f"tool_calls: [{names}]"
    return text, in_tok, out_tok
