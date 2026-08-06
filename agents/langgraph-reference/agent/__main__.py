"""CLI: python -m agent run --repo <owner/name> --traces-dir <dir>"""

from __future__ import annotations

import argparse
import os
import sys
import uuid
from typing import Optional, Sequence

from .app import run_triage
from .github import GitHubClient
from .graph import DEFAULT_ITERATION_CAP
from .llm import DEFAULT_MODEL, MissingAPIKey, build_llm


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="agent", description="trazo LangGraph reference agent")
    sub = parser.add_subparsers(dest="command", required=True)

    run_p = sub.add_parser("run", help="triage a GitHub repo and emit a trace")
    run_p.add_argument("--repo", required=True, help="target repository as owner/name")
    run_p.add_argument("--traces-dir", required=True, help="directory to write the trace into")
    run_p.add_argument("--model", default=DEFAULT_MODEL, help=f"Gemini model (default {DEFAULT_MODEL})")
    run_p.add_argument("--api-key", default=None, help="Gemini API key (default: GEMINI_API_KEY env)")
    run_p.add_argument("--github-token", default=None, help="GitHub token (default: GITHUB_TOKEN env)")
    run_p.add_argument(
        "--iteration-cap", type=int, default=DEFAULT_ITERATION_CAP,
        help=f"max agent turns before forcing a stop (default {DEFAULT_ITERATION_CAP})",
    )
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = _build_parser().parse_args(argv)
    if args.command == "run":
        return _cmd_run(args)
    return 1


def _cmd_run(args: argparse.Namespace) -> int:
    try:
        llm = build_llm(args.model, api_key=args.api_key)
    except MissingAPIKey as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    source = GitHubClient(token=args.github_token or os.environ.get("GITHUB_TOKEN"))
    run_id = f"triage-{args.repo.replace('/', '-')}-{uuid.uuid4().hex[:8]}"

    result = run_triage(
        args.repo,
        args.traces_dir,
        llm=llm,
        source=source,
        model=args.model,
        run_id=run_id,
        iteration_cap=args.iteration_cap,
    )

    print(f"run_id: {result.run_id}")
    print(f"trace:  {result.trace_path}")
    print(f"steps:  {result.steps}")
    print("--- triage report ---")
    print(result.report or "(no report produced)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
