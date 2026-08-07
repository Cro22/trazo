"""TraceRecorder: build a Run step by step and flush it to disk.

Correlation note: tool_call and tool_result are paired by an explicit call id.
record_tool_call returns a ToolCall handle carrying a generated id, which is
written to both the call and its result as toolCallId, so pairing is precise even
when the same tool is called several times. record_tool_result also accepts a
bare tool name, in which case the Go core falls back to name/FIFO pairing (see
evaluator/toolcalls.go and ../docs/trace-schema.md).
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable, Optional, Union

from .models import Run, Step, StepType

# SCHEMA_VERSION mirrors trajectory.SchemaVersion in the Go core and is the
# default version stamped on emitted traces. Keep the two in sync: the Go loader
# rejects a trace whose major differs from its supported major. Bump the minor
# for additive changes, the major for breaking ones.
SCHEMA_VERSION = "0.1.0"


@dataclass(frozen=True)
class ToolCall:
    """Handle returned by record_tool_call, used to pair the later result. The id
    is written to both the call and its result as toolCallId, giving precise
    pairing even when the same tool is called several times."""

    tool: str
    step_index: int
    id: str


Clock = Callable[[], datetime]


def _utc_now() -> datetime:
    return datetime.now(timezone.utc)


class TraceRecorder:
    """Accumulates steps for one run and serializes to trazo JSON.

    Pass an explicit clock (a zero-arg callable returning an aware datetime) for
    deterministic output in tests; otherwise it uses the wall clock in UTC.
    """

    def __init__(
        self,
        run_id: str,
        agent: str,
        version: str = SCHEMA_VERSION,
        *,
        clock: Optional[Clock] = None,
        start_time: Optional[datetime] = None,
    ) -> None:
        self.run_id = run_id
        self.agent = agent
        self.version = version
        self._clock: Clock = clock or _utc_now
        self.start_time: datetime = start_time or self._clock()
        self.steps: list[Step] = []
        self._tool_call_seq = 0

    def _stamp(self, timestamp: Optional[datetime]) -> datetime:
        return timestamp if timestamp is not None else self._clock()

    def record_node_transition(
        self,
        node: str,
        *,
        duration_ms: int = 0,
        timestamp: Optional[datetime] = None,
    ) -> None:
        self.steps.append(
            Step(
                type=StepType.NODE_TRANSITION,
                timestamp=self._stamp(timestamp),
                node=node,
                duration_ms=duration_ms,
            )
        )

    def record_llm_call(
        self,
        llm: str,
        *,
        input: Optional[Any] = None,
        output: Optional[Any] = None,
        cost: float = 0.0,
        input_tokens: int = 0,
        output_tokens: int = 0,
        duration_ms: int = 0,
        timestamp: Optional[datetime] = None,
    ) -> None:
        self.steps.append(
            Step(
                type=StepType.LLM_CALL,
                timestamp=self._stamp(timestamp),
                llm=llm,
                input=input,
                output=output,
                cost=cost,
                input_tokens=input_tokens,
                output_tokens=output_tokens,
                duration_ms=duration_ms,
            )
        )

    def record_tool_call(
        self,
        tool: str,
        *,
        input: Optional[Any] = None,
        duration_ms: int = 0,
        timestamp: Optional[datetime] = None,
        call_id: Optional[str] = None,
    ) -> ToolCall:
        """Record a tool call. A toolCallId is generated when not supplied, so the
        emitted trace always pairs precisely; pass call_id to reuse the model's
        own id when available."""
        if call_id is None:
            self._tool_call_seq += 1
            call_id = f"call-{self._tool_call_seq}"
        self.steps.append(
            Step(
                type=StepType.TOOL_CALL,
                timestamp=self._stamp(timestamp),
                tool=tool,
                tool_call_id=call_id,
                input=input,
                duration_ms=duration_ms,
            )
        )
        return ToolCall(tool=tool, step_index=len(self.steps) - 1, id=call_id)

    def record_tool_result(
        self,
        call: Union[ToolCall, str],
        *,
        output: Optional[Any] = None,
        error: Optional[str] = None,
        duration_ms: int = 0,
        timestamp: Optional[datetime] = None,
    ) -> None:
        """Record a tool result. Passing the ToolCall handle carries its
        toolCallId onto the result for id-based pairing; a bare tool name pairs by
        name/order instead."""
        if isinstance(call, ToolCall):
            tool = call.tool
            call_id: Optional[str] = call.id
        else:
            tool = call
            call_id = None
        self.steps.append(
            Step(
                type=StepType.TOOL_RESULT,
                timestamp=self._stamp(timestamp),
                tool=tool,
                tool_call_id=call_id,
                output=output,
                error=error,
                duration_ms=duration_ms,
            )
        )

    def to_run(self, *, end_time: Optional[datetime] = None) -> Run:
        """Build the Run envelope. end_time defaults to now (clamped to be at
        least the last step's timestamp, so endTime is never before the trace)."""
        end = end_time if end_time is not None else self._clock()
        if self.steps:
            last = self.steps[-1].timestamp
            if end < last:
                end = last
        if end < self.start_time:
            end = self.start_time
        return Run(
            id=self.run_id,
            agent=self.agent,
            version=self.version,
            start_time=self.start_time,
            end_time=end,
            steps=list(self.steps),
        )

    def to_json(self, *, end_time: Optional[datetime] = None, indent: int = 2) -> str:
        return json.dumps(self.to_run(end_time=end_time).to_dict(), indent=indent)

    def flush(self, directory: Union[str, Path], *, end_time: Optional[datetime] = None) -> Path:
        """Write one file, <run_id>.json, into directory. Returns its path."""
        out_dir = Path(directory)
        out_dir.mkdir(parents=True, exist_ok=True)
        path = out_dir / f"{self.run_id}.json"
        path.write_text(self.to_json(end_time=end_time), encoding="utf-8")
        return path
