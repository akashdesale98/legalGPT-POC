"""LLM-as-Judge evaluation script for LegalGPT POC.

Reads the latest results JSON from eval/results/, scores each per_query entry
using Claude claude-sonnet-4-5-20251001 on:
  - legal_accuracy (1-5)
  - citation_quality (1-5)
  - hallucination_risk (1-5, lower is better)
  + reasoning string

Overwrites the results file with judge scores appended to each per_query entry.

Usage:
    python eval/judge_prompt.py
    python eval/judge_prompt.py --results eval/results/2024-01-01_12-00.json
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from pathlib import Path
from typing import Any, Optional

from dotenv import load_dotenv
from rich.console import Console
from rich.progress import track

load_dotenv()

console = Console()

RESULTS_DIR = Path(__file__).parent / "results"
JUDGE_MODEL = "claude-sonnet-4-5-20251001"
RETRY_ATTEMPTS = 3
RETRY_DELAY = 2.0  # seconds


JUDGE_SYSTEM_PROMPT = """\
You are an expert Indian legal evaluator assessing the quality of AI-generated
legal answers about Indian law (BNS, BNSS, BSA, and the Constitution of India).

Your task is to evaluate a generated answer against a reference answer for a given query.

You will provide scores on three dimensions:

1. legal_accuracy (1-5):
   1 = Completely wrong or misleading legal information
   2 = Mostly incorrect with some correct elements
   3 = Partially correct with notable gaps or errors
   4 = Mostly correct with minor inaccuracies
   5 = Accurate and complete legal information

2. citation_quality (1-5):
   1 = No citations or completely wrong citations
   2 = Vague or mostly incorrect citations
   3 = Some correct citations but incomplete
   4 = Good citations with minor gaps
   5 = Precise, complete, and correct citations to specific sections/articles

3. hallucination_risk (1-5, LOWER IS BETTER):
   1 = No hallucinations — every claim is supported
   2 = Very low risk — minor unsupported claims
   3 = Moderate risk — some unsupported or speculative claims
   4 = High risk — significant unsupported assertions
   5 = Very high risk — fabricated or clearly incorrect legal claims

Respond ONLY with valid JSON in this exact format:
{
  "legal_accuracy": <int 1-5>,
  "citation_quality": <int 1-5>,
  "hallucination_risk": <int 1-5>,
  "reasoning": "<brief explanation of scores, max 200 words>"
}
"""


def _build_judge_prompt(
    query: str,
    reference_answer: str,
    generated_answer: str,
    retrieved_contexts: list[str],
) -> str:
    ctx_summary = (
        "\n".join(f"  - {c}" for c in retrieved_contexts[:5])
        if retrieved_contexts
        else "  (none)"
    )
    return f"""\
Query: {query}

Reference Answer:
{reference_answer}

Generated Answer:
{generated_answer}

Retrieved Source Citations:
{ctx_summary}

Please evaluate the generated answer against the reference answer.
"""


def _judge_entry(
    client: Any,
    entry: dict[str, Any],
) -> dict[str, Any]:
    """Score a single per_query entry. Returns judge scores dict."""
    query = entry.get("query", "")
    reference = entry.get("reference_answer", "")
    generated = entry.get("generated_answer", "")
    contexts = entry.get("retrieved_contexts", [])

    # Skip abstention queries — nothing meaningful to judge on hallucination/accuracy
    if entry.get("query_type") == "abstention":
        return {
            "legal_accuracy": None,
            "citation_quality": None,
            "hallucination_risk": None,
            "reasoning": "Abstention query — judge scoring not applicable.",
        }

    # Skip if there was an error or no answer
    if entry.get("error") or not generated.strip():
        return {
            "legal_accuracy": None,
            "citation_quality": None,
            "hallucination_risk": None,
            "reasoning": "No generated answer available for judging.",
        }

    prompt = _build_judge_prompt(query, reference, generated, contexts)

    for attempt in range(RETRY_ATTEMPTS):
        try:
            response = client.messages.create(
                model=JUDGE_MODEL,
                max_tokens=512,
                system=JUDGE_SYSTEM_PROMPT,
                messages=[{"role": "user", "content": prompt}],
            )
            raw_text = response.content[0].text.strip()
            scores = json.loads(raw_text)
            # Validate expected keys
            for key in ("legal_accuracy", "citation_quality", "hallucination_risk", "reasoning"):
                if key not in scores:
                    raise ValueError(f"Missing key in judge response: {key}")
            return {
                "legal_accuracy": int(scores["legal_accuracy"]),
                "citation_quality": int(scores["citation_quality"]),
                "hallucination_risk": int(scores["hallucination_risk"]),
                "reasoning": str(scores["reasoning"]),
            }
        except (json.JSONDecodeError, KeyError, ValueError) as exc:
            console.print(
                f"[yellow]Judge parse error (attempt {attempt + 1}): {exc}[/yellow]"
            )
            if attempt < RETRY_ATTEMPTS - 1:
                time.sleep(RETRY_DELAY)
        except Exception as exc:  # noqa: BLE001
            console.print(
                f"[yellow]Judge API error (attempt {attempt + 1}): {exc}[/yellow]"
            )
            if attempt < RETRY_ATTEMPTS - 1:
                time.sleep(RETRY_DELAY)

    return {
        "legal_accuracy": None,
        "citation_quality": None,
        "hallucination_risk": None,
        "reasoning": "Judge scoring failed after all retries.",
    }


def _find_latest_results() -> Optional[Path]:
    """Return the most recently modified results JSON file."""
    results = sorted(
        RESULTS_DIR.glob("*.json"),
        key=lambda p: p.stat().st_mtime,
        reverse=True,
    )
    return results[0] if results else None


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Run LLM-as-judge scoring on eval results"
    )
    parser.add_argument(
        "--results",
        default=None,
        help="Path to results JSON (default: latest in eval/results/)",
    )
    args = parser.parse_args()

    api_key = os.getenv("ANTHROPIC_API_KEY")
    if not api_key:
        console.print("[red]ANTHROPIC_API_KEY not set — cannot run judge.[/red]")
        sys.exit(1)

    try:
        import anthropic  # type: ignore[import-untyped]
    except ImportError:
        console.print("[red]anthropic package not installed. Run: pip install anthropic[/red]")
        sys.exit(1)

    client = anthropic.Anthropic(api_key=api_key)

    if args.results:
        results_path = Path(args.results)
    else:
        results_path = _find_latest_results()

    if not results_path or not results_path.exists():
        console.print(
            f"[red]No results file found. Run ragas_eval.py first.[/red]"
        )
        sys.exit(1)

    console.print(f"[green]Loading results from {results_path}[/green]")
    with results_path.open("r", encoding="utf-8") as fh:
        results = json.load(fh)

    per_query: list[dict[str, Any]] = results.get("per_query", [])
    console.print(f"Scoring {len(per_query)} entries with {JUDGE_MODEL}...")

    scored = 0
    skipped = 0
    total_legal_accuracy: list[int] = []
    total_citation_quality: list[int] = []
    total_hallucination_risk: list[int] = []

    for entry in track(per_query, description="Judging..."):
        judge_scores = _judge_entry(client, entry)
        entry["judge_legal_accuracy"] = judge_scores["legal_accuracy"]
        entry["judge_citation_quality"] = judge_scores["citation_quality"]
        entry["judge_hallucination_risk"] = judge_scores["hallucination_risk"]
        entry["judge_reasoning"] = judge_scores["reasoning"]

        if judge_scores["legal_accuracy"] is not None:
            total_legal_accuracy.append(judge_scores["legal_accuracy"])
            total_citation_quality.append(judge_scores["citation_quality"])
            total_hallucination_risk.append(judge_scores["hallucination_risk"])
            scored += 1
        else:
            skipped += 1

    # Update aggregate with judge stats
    if total_legal_accuracy:
        results["judge_aggregate"] = {
            "avg_legal_accuracy": sum(total_legal_accuracy) / len(total_legal_accuracy),
            "avg_citation_quality": sum(total_citation_quality) / len(total_citation_quality),
            "avg_hallucination_risk": sum(total_hallucination_risk) / len(total_hallucination_risk),
            "scored_count": scored,
            "skipped_count": skipped,
        }
        console.print(f"\n[bold]Judge Aggregate:[/bold]")
        console.print(
            f"  Legal Accuracy:      {results['judge_aggregate']['avg_legal_accuracy']:.2f}/5"
        )
        console.print(
            f"  Citation Quality:    {results['judge_aggregate']['avg_citation_quality']:.2f}/5"
        )
        console.print(
            f"  Hallucination Risk:  {results['judge_aggregate']['avg_hallucination_risk']:.2f}/5 (lower=better)"
        )

    # Overwrite results file with judge scores included
    with results_path.open("w", encoding="utf-8") as fh:
        json.dump(results, fh, indent=2, ensure_ascii=False)

    console.print(
        f"\n[green]Judge scores appended to {results_path} "
        f"({scored} scored, {skipped} skipped).[/green]"
    )


if __name__ == "__main__":
    main()
