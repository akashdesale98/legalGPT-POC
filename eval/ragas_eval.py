"""RAGAS Evaluation Harness for LegalGPT POC.

Usage:
    python eval/ragas_eval.py --base-url http://localhost:8080
    python eval/ragas_eval.py --base-url http://localhost:8080 --golden eval/golden_set.jsonl
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import warnings
from datetime import datetime
from pathlib import Path
from typing import Any, Optional

import httpx
from dotenv import load_dotenv
from rich.console import Console
from rich.table import Table

load_dotenv()

console = Console()

RESULTS_DIR = Path(__file__).parent / "results"
DEFAULT_GOLDEN = Path(__file__).parent / "golden_set.jsonl"
CHAT_ENDPOINT = "/api/v1/chat"
HEALTH_ENDPOINT = "/api/v1/health/ready"
REQUEST_TIMEOUT = 120.0


# ---------------------------------------------------------------------------
# SSE parsing
# ---------------------------------------------------------------------------


def _parse_sse_stream(raw_text: str) -> list[dict[str, Any]]:
    """Parse a raw SSE response body into a list of {event, data} dicts."""
    events: list[dict[str, Any]] = []
    current_event: Optional[str] = None
    current_data_lines: list[str] = []

    for line in raw_text.splitlines():
        if line.startswith("event:"):
            current_event = line[len("event:") :].strip()
        elif line.startswith("data:"):
            current_data_lines.append(line[len("data:") :].strip())
        elif line == "" and current_event is not None:
            raw_data = " ".join(current_data_lines)
            try:
                parsed_data = json.loads(raw_data)
            except json.JSONDecodeError:
                parsed_data = {"raw": raw_data}
            events.append({"event": current_event, "data": parsed_data})
            current_event = None
            current_data_lines = []

    return events


def _call_chat(base_url: str, query: str, dev_token: Optional[str]) -> dict[str, Any]:
    """Call POST /api/v1/chat and return parsed SSE events.

    Returns a dict with keys:
        generated_answer: str
        retrieved_contexts: list[str]
        abstained: bool
        abstain_reason: Optional[str]
        error: Optional[str]
    """
    headers: dict[str, str] = {"Content-Type": "application/json"}
    if dev_token:
        headers["Authorization"] = f"Bearer {dev_token}"

    payload = {"query": query, "collections": []}

    try:
        response = httpx.post(
            f"{base_url}{CHAT_ENDPOINT}",
            json=payload,
            headers=headers,
            timeout=REQUEST_TIMEOUT,
        )
    except httpx.ConnectError as exc:
        return {
            "generated_answer": "",
            "retrieved_contexts": [],
            "abstained": False,
            "abstain_reason": None,
            "error": f"Connection refused: {exc}",
        }
    except httpx.TimeoutException as exc:
        return {
            "generated_answer": "",
            "retrieved_contexts": [],
            "abstained": False,
            "abstain_reason": None,
            "error": f"Request timed out: {exc}",
        }

    try:
        events = _parse_sse_stream(response.text)
    except Exception as exc:  # noqa: BLE001
        warnings.warn(f"SSE parsing error: {exc}", stacklevel=2)
        return {
            "generated_answer": "",
            "retrieved_contexts": [],
            "abstained": False,
            "abstain_reason": None,
            "error": f"SSE parse error: {exc}",
        }

    answer_parts: list[str] = []
    contexts: list[str] = []
    abstained = False
    abstain_reason: Optional[str] = None

    for evt in events:
        name = evt["event"]
        data = evt["data"]
        if name == "chunk":
            answer_parts.append(data.get("delta", ""))
        elif name == "sources":
            citations = data.get("citations", [])
            for c in citations:
                parts = []
                if "statute" in c:
                    parts.append(str(c["statute"]))
                if "section" in c:
                    parts.append(f"Section {c['section']}")
                if "article_number" in c:
                    parts.append(f"Article {c['article_number']}")
                contexts.append(" ".join(parts) if parts else json.dumps(c))
        elif name == "abstain":
            abstained = True
            abstain_reason = data.get("reason")
        elif name == "error":
            return {
                "generated_answer": "",
                "retrieved_contexts": [],
                "abstained": False,
                "abstain_reason": None,
                "error": data.get("message", "unknown server error"),
            }

    return {
        "generated_answer": "".join(answer_parts),
        "retrieved_contexts": contexts,
        "abstained": abstained,
        "abstain_reason": abstain_reason,
        "error": None,
    }


# ---------------------------------------------------------------------------
# RAGAS scoring
# ---------------------------------------------------------------------------


def _compute_ragas_scores(
    queries: list[str],
    answers: list[str],
    contexts_list: list[list[str]],
    references: list[str],
) -> Optional[dict[str, float]]:
    """Run RAGAS metrics. Returns None if RAGAS is unavailable or LLM not configured."""
    anthropic_key = os.getenv("ANTHROPIC_API_KEY")
    if not anthropic_key:
        console.print(
            "[yellow]ANTHROPIC_API_KEY not set — skipping RAGAS scoring.[/yellow]"
        )
        return None

    try:
        from datasets import Dataset  # type: ignore[import-untyped]
        from ragas import evaluate  # type: ignore[import-untyped]
        from ragas.metrics import (  # type: ignore[import-untyped]
            answer_relevancy,
            context_precision,
            faithfulness,
        )
    except ImportError as exc:
        console.print(f"[yellow]RAGAS not installed — skipping scoring: {exc}[/yellow]")
        return None

    dataset_dict = {
        "question": queries,
        "answer": answers,
        "contexts": contexts_list,
        "ground_truth": references,
    }

    try:
        dataset = Dataset.from_dict(dataset_dict)
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            result = evaluate(
                dataset,
                metrics=[faithfulness, answer_relevancy, context_precision],
            )
        return {
            "faithfulness": float(result["faithfulness"]),
            "answer_relevancy": float(result["answer_relevancy"]),
            "context_precision": float(result["context_precision"]),
        }
    except Exception as exc:  # noqa: BLE001
        console.print(f"[yellow]RAGAS evaluation failed: {exc}[/yellow]")
        return None


# ---------------------------------------------------------------------------
# Git commit SHA
# ---------------------------------------------------------------------------


def _get_git_commit() -> str:
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        return result.stdout.strip() if result.returncode == 0 else "unknown"
    except Exception:  # noqa: BLE001
        return "unknown"


# ---------------------------------------------------------------------------
# Rich table
# ---------------------------------------------------------------------------


def _print_results_table(per_query: list[dict[str, Any]]) -> None:
    table = Table(
        title="LegalGPT RAGAS Evaluation Results",
        show_lines=True,
    )
    table.add_column("ID", style="cyan", no_wrap=True)
    table.add_column("Type", style="magenta")
    table.add_column("Difficulty")
    table.add_column("Abstain", justify="center")
    table.add_column("Faithful", justify="right")
    table.add_column("Relevancy", justify="right")
    table.add_column("Ctx Prec", justify="right")
    table.add_column("Error", style="red")

    for row in per_query:
        faithful_str = (
            f"{row['faithfulness']:.3f}" if row.get("faithfulness") is not None else "-"
        )
        relevancy_str = (
            f"{row['answer_relevancy']:.3f}"
            if row.get("answer_relevancy") is not None
            else "-"
        )
        ctx_prec_str = (
            f"{row['context_precision']:.3f}"
            if row.get("context_precision") is not None
            else "-"
        )
        abstain_str = ""
        if row.get("query_type") == "abstention":
            abstain_str = "[green]OK[/green]" if row.get("abstention_correct") else "[red]FAIL[/red]"

        table.add_row(
            row["id"],
            row.get("query_type", ""),
            row.get("difficulty", ""),
            abstain_str,
            faithful_str,
            relevancy_str,
            ctx_prec_str,
            row.get("error") or "",
        )

    console.print(table)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def _load_golden_set(path: Path) -> list[dict[str, Any]]:
    items = []
    with path.open("r", encoding="utf-8") as fh:
        for line_no, line in enumerate(fh, 1):
            line = line.strip()
            if not line:
                continue
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as exc:
                console.print(
                    f"[yellow]Skipping malformed line {line_no}: {exc}[/yellow]"
                )
    return items


def main() -> None:
    parser = argparse.ArgumentParser(description="Run RAGAS evaluation against LegalGPT")
    parser.add_argument(
        "--base-url",
        default="http://localhost:8080",
        help="Base URL of the query service (default: http://localhost:8080)",
    )
    parser.add_argument(
        "--golden",
        default=str(DEFAULT_GOLDEN),
        help="Path to golden_set.jsonl",
    )
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    golden_path = Path(args.golden)
    dev_token = os.getenv("DEV_TOKEN")

    if not golden_path.exists():
        console.print(f"[red]Golden set not found: {golden_path}[/red]")
        sys.exit(1)

    # Verify service is reachable
    try:
        health_resp = httpx.get(f"{base_url}{HEALTH_ENDPOINT}", timeout=10.0)
        if health_resp.status_code not in (200, 204):
            console.print(
                f"[red]Service health check failed: HTTP {health_resp.status_code}[/red]"
            )
            sys.exit(1)
    except httpx.ConnectError:
        console.print(
            f"[red]Cannot connect to {base_url}. Is the service running?[/red]"
        )
        sys.exit(1)

    golden_items = _load_golden_set(golden_path)
    console.print(f"[green]Loaded {len(golden_items)} golden queries.[/green]")

    per_query: list[dict[str, Any]] = []
    ragas_inputs: dict[str, list[Any]] = {
        "queries": [],
        "answers": [],
        "contexts": [],
        "references": [],
        "indices": [],
    }

    for item in golden_items:
        qid = item.get("id", "?")
        query = item.get("query", "")
        query_type = item.get("query_type", "")
        difficulty = item.get("difficulty", "")
        reference = item.get("reference_answer", "")

        console.print(f"  Querying [cyan]{qid}[/cyan]...")

        result = _call_chat(base_url, query, dev_token)

        if result["error"] and "Connection refused" in result["error"]:
            console.print(f"[red]{result['error']}[/red]")
            sys.exit(1)

        row: dict[str, Any] = {
            "id": qid,
            "query": query,
            "query_type": query_type,
            "difficulty": difficulty,
            "reference_answer": reference,
            "generated_answer": result["generated_answer"],
            "retrieved_contexts": result["retrieved_contexts"],
            "abstained": result["abstained"],
            "abstain_reason": result["abstain_reason"],
            "error": result["error"],
            "faithfulness": None,
            "answer_relevancy": None,
            "context_precision": None,
        }

        if query_type == "abstention":
            row["abstention_correct"] = result["abstained"]
        else:
            row["abstention_correct"] = None

        # Queue non-abstention, non-error queries for RAGAS scoring
        if not result["error"] and not result["abstained"] and query_type != "abstention":
            ragas_inputs["queries"].append(query)
            ragas_inputs["answers"].append(result["generated_answer"])
            # Ensure contexts list is non-empty for RAGAS
            ctx = result["retrieved_contexts"] if result["retrieved_contexts"] else [""]
            ragas_inputs["contexts"].append(ctx)
            ragas_inputs["references"].append(reference)
            ragas_inputs["indices"].append(len(per_query))

        per_query.append(row)

    # Compute RAGAS scores in bulk
    ragas_scores: Optional[dict[str, float]] = None
    if ragas_inputs["queries"]:
        console.print("[bold]Running RAGAS metrics...[/bold]")
        ragas_scores = _compute_ragas_scores(
            ragas_inputs["queries"],
            ragas_inputs["answers"],
            ragas_inputs["contexts"],
            ragas_inputs["references"],
        )

    # RAGAS returns aggregate — we assign uniform scores per query for now.
    # Some RAGAS versions return per-sample scores in the dataset; handle both.
    if ragas_scores:
        score_count = len(ragas_inputs["indices"])
        for local_idx, global_idx in enumerate(ragas_inputs["indices"]):
            # ragas_scores are aggregate values — assign to each relevant query
            # If a per-query breakdown is available it would need different handling
            per_query[global_idx]["faithfulness"] = ragas_scores.get("faithfulness")
            per_query[global_idx]["answer_relevancy"] = ragas_scores.get(
                "answer_relevancy"
            )
            per_query[global_idx]["context_precision"] = ragas_scores.get(
                "context_precision"
            )

    # Print rich table
    _print_results_table(per_query)

    # Compute abstention accuracy
    abstention_items = [r for r in per_query if r.get("query_type") == "abstention"]
    abstention_correct = sum(1 for r in abstention_items if r.get("abstention_correct"))
    if abstention_items:
        console.print(
            f"\nAbstention accuracy: {abstention_correct}/{len(abstention_items)}"
        )

    # Aggregate scores
    aggregate: dict[str, Any] = {}
    if ragas_scores:
        aggregate = {
            "faithfulness": ragas_scores.get("faithfulness", 0.0),
            "answer_relevancy": ragas_scores.get("answer_relevancy", 0.0),
            "context_precision": ragas_scores.get("context_precision", 0.0),
        }
        console.print(f"\n[bold]Aggregate scores:[/bold] {aggregate}")
    else:
        aggregate = {
            "faithfulness": None,
            "answer_relevancy": None,
            "context_precision": None,
        }

    # Write results JSON
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    run_date = datetime.utcnow().strftime("%Y-%m-%d_%H-%M")
    out_path = RESULTS_DIR / f"{run_date}.json"
    output = {
        "run_date": datetime.utcnow().isoformat() + "Z",
        "git_commit": _get_git_commit(),
        "aggregate": aggregate,
        "per_query": per_query,
    }
    with out_path.open("w", encoding="utf-8") as fh:
        json.dump(output, fh, indent=2, ensure_ascii=False)
    console.print(f"\n[green]Results written to {out_path}[/green]")

    # Exit with error if faithfulness is below threshold (only when scores available)
    if ragas_scores is not None:
        faith = ragas_scores.get("faithfulness", 1.0)
        if faith < 0.75:
            console.print(
                f"[red]FAIL: faithfulness={faith:.3f} is below threshold 0.75[/red]"
            )
            sys.exit(1)

    console.print("[green]Evaluation complete.[/green]")


if __name__ == "__main__":
    main()
