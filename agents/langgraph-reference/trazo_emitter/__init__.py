"""Trazo trace emitter.

A tiny, dependency-free library that builds trazo-format Run/Step objects and
serializes them to the JSON the Go core consumes. No LangGraph dependency here;
this layer is pure data. See ../docs/trace-schema.md for the format.
"""

from .models import Run, Step, StepType, to_rfc3339
from .recorder import SCHEMA_VERSION, ToolCall, TraceRecorder

__all__ = [
    "Run",
    "Step",
    "StepType",
    "TraceRecorder",
    "ToolCall",
    "SCHEMA_VERSION",
    "to_rfc3339",
]
