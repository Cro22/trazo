"""The triage graph: an LLM node, a tool node, and a capped conditional loop.

Shape:
    START -> agent -> (tool_calls?) -> tools -> agent -> ... -> END
The conditional edge routes to the tool node while the model requests tools and
the iteration cap has not been reached; otherwise it ends. The cap is the
runaway-loop guard exercised in M4.
"""

from __future__ import annotations

from typing import Annotated, Any, Sequence, TypedDict

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AnyMessage
from langchain_core.tools import BaseTool
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages
from langgraph.prebuilt import ToolNode

DEFAULT_ITERATION_CAP = 5

SYSTEM_PROMPT = (
    "You triage GitHub issues. Given a repository (owner/name):\n"
    "1. Call fetch_issues once to get the open issues.\n"
    "2. Call classify_issue for each issue to label it.\n"
    "3. Then write a short triage report (one line per issue plus a summary) "
    "as your final answer, with no further tool calls.\n"
    "Be concise. Do not loop."
)


class TriageState(TypedDict):
    messages: Annotated[list[AnyMessage], add_messages]
    iterations: int


def build_agent(
    llm: BaseChatModel,
    tools: Sequence[BaseTool],
    *,
    iteration_cap: int = DEFAULT_ITERATION_CAP,
) -> Any:
    """Compile the triage graph. The returned object is invoked with a config
    carrying a thread_id (checkpointing) and the tracing callback."""
    llm_with_tools = llm.bind_tools(list(tools))

    def agent_node(state: TriageState) -> dict[str, Any]:
        response = llm_with_tools.invoke(state["messages"])
        return {
            "messages": [response],
            "iterations": state.get("iterations", 0) + 1,
        }

    def route(state: TriageState) -> str:
        if state.get("iterations", 0) >= iteration_cap:
            return END
        last = state["messages"][-1]
        if getattr(last, "tool_calls", None):
            return "tools"
        return END

    graph = StateGraph(TriageState)
    graph.add_node("agent", agent_node)
    # handle_tool_errors=True turns a raised tool error into a ToolMessage so the
    # agent can handle it and the graph keeps running; on_tool_error still fires,
    # so the trace records the errored tool_result.
    graph.add_node("tools", ToolNode(list(tools), handle_tool_errors=True))
    graph.add_edge(START, "agent")
    graph.add_conditional_edges("agent", route, {"tools": "tools", END: END})
    graph.add_edge("tools", "agent")
    return graph.compile(checkpointer=InMemorySaver())
