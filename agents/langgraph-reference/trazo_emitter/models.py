"""Data model mirroring the Go trajectory types.

Field names and serialization match trajectory/steps.go exactly:
- durationMs is always serialized (the Go tag has no omitempty).
- Every other optional field is omitted when empty/zero, matching Go omitempty.
- Timestamps are RFC3339 with a Z suffix (UTC), as in the fixtures.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Optional


class StepType(str, Enum):
    """The four step types accepted by the Go core (trajectory/steps.go)."""

    LLM_CALL = "llm_call"
    TOOL_CALL = "tool_call"
    TOOL_RESULT = "tool_result"
    NODE_TRANSITION = "node_transition"


def to_rfc3339(dt: datetime) -> str:
    """Format a datetime as RFC3339 UTC with a Z suffix.

    Naive datetimes are assumed to be UTC. Go's time parser accepts the numeric
    offset too, but the fixtures use Z, so we normalize to that.
    """
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


@dataclass
class Step:
    """A single trajectory step. Use the type-specific fields per StepType."""

    type: StepType
    timestamp: datetime
    duration_ms: int = 0
    llm: Optional[str] = None
    tool: Optional[str] = None
    tool_call_id: Optional[str] = None
    node: Optional[str] = None
    input: Optional[Any] = None
    output: Optional[Any] = None
    cost: float = 0.0
    input_tokens: int = 0
    output_tokens: int = 0
    error: Optional[str] = None

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "type": self.type.value,
            "timestamp": to_rfc3339(self.timestamp),
        }
        if self.llm:
            d["llm"] = self.llm
        if self.tool:
            d["tool"] = self.tool
        if self.tool_call_id:
            d["toolCallId"] = self.tool_call_id
        if self.node:
            d["node"] = self.node
        if self.input is not None:
            d["input"] = self.input
        if self.output is not None:
            d["output"] = self.output
        if self.cost:
            d["cost"] = self.cost
        if self.input_tokens:
            d["inputTokens"] = self.input_tokens
        if self.output_tokens:
            d["outputTokens"] = self.output_tokens
        # durationMs has no omitempty on the Go side: always serialize it.
        d["durationMs"] = self.duration_ms
        if self.error:
            d["error"] = self.error
        return d


@dataclass
class Run:
    """A run envelope: metadata plus an ordered list of steps."""

    id: str
    agent: str
    version: str
    start_time: datetime
    end_time: datetime
    steps: list[Step] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "agent": self.agent,
            "version": self.version,
            "startTime": to_rfc3339(self.start_time),
            "endTime": to_rfc3339(self.end_time),
            "steps": [s.to_dict() for s in self.steps],
        }
