#!/usr/bin/env python3
"""
RAGAS evaluation pipeline for Personal AI Assistant.

Calls PAA API (/v1/rag/query), collects answers + sources,
evaluates via RAGAS with Ollama as LLM judge.

Usage:
    python3 scripts/eval/ragas_eval.py \
        --api-url http://localhost:8080 \
        --cases scripts/eval/cases.example.jsonl \
        --k 5 \
        --out ./tmp/eval/ragas_report.json

Environment:
    RAGAS_JUDGE_MODEL   Ollama model for evaluation (default: qwen3.5:9b)
    RAGAS_JUDGE_URL     Ollama OpenAI-compat URL (default: http://localhost:11434/v1)
"""

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

import warnings
warnings.filterwarnings("ignore", category=DeprecationWarning)

import requests
from ragas import evaluate
from ragas.metrics import (
    answer_relevancy,
    answer_similarity,
    context_precision,
    context_recall,
    faithfulness,
)

try:
    from ragas.metrics import answer_correctness
except ImportError:
    answer_correctness = None

from datasets import Dataset
from langchain_openai import ChatOpenAI


def parse_args():
    parser = argparse.ArgumentParser(description="RAGAS evaluation for PAA")
    parser.add_argument("--api-url", default="http://localhost:8080", help="PAA API URL")
    parser.add_argument("--cases", default="scripts/eval/cases.example.jsonl", help="JSONL cases file")
    parser.add_argument("--k", type=int, default=5, help="Top-K for retrieval")
    parser.add_argument("--out", default="./tmp/eval/ragas_report.json", help="Output report JSON")
    return parser.parse_args()


def load_cases(path: str) -> list[dict]:
    cases = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            cases.append(json.loads(line))
    return cases


def query_paa(api_url: str, question: str, limit: int) -> dict:
    """Call PAA /v1/rag/query and return response JSON."""
    resp = requests.post(
        f"{api_url}/v1/rag/query",
        json={"question": question, "limit": limit},
        headers={"Content-Type": "application/json"},
        timeout=60,
    )
    resp.raise_for_status()
    return resp.json()


def build_ragas_dataset(cases: list[dict], api_url: str, k: int) -> tuple[Dataset, list[dict]]:
    """Query PAA for each case and build a RAGAS-compatible Dataset."""
    questions = []
    answers = []
    contexts = []
    ground_truths = []
    case_details = []

    for i, case in enumerate(cases):
        question = case["question"]
        case_id = case.get("id", f"case_{i}")
        gt = case.get("ground_truth", "")

        print(f"  [{i+1}/{len(cases)}] {case_id}: {question[:60]}...", end=" ", flush=True)

        try:
            result = query_paa(api_url, question, k)
        except Exception as e:
            print(f"SKIP ({e})")
            continue

        answer_text = result.get("text", "")
        sources = result.get("sources", [])
        context_texts = [s.get("text", "") for s in sources if s.get("text")]
        actual_filenames = list({s.get("filename", "") for s in sources if s.get("filename")})

        questions.append(question)
        answers.append(answer_text)
        contexts.append(context_texts if context_texts else [""])
        ground_truths.append(gt if gt else "")

        case_details.append({
            "id": case_id,
            "question": question,
            "answer": answer_text[:200],
            "context_count": len(context_texts),
            "actual_filenames": actual_filenames,
            "expected_filenames": case.get("expected_filenames", []),
            "has_ground_truth": bool(gt),
        })

        print("OK")

    dataset = Dataset.from_dict({
        "question": questions,
        "answer": answers,
        "contexts": contexts,
        "ground_truth": ground_truths,
    })

    return dataset, case_details


def run_evaluation(dataset: Dataset, llm) -> dict:
    """Run RAGAS evaluation and return scores."""
    metrics = [
        faithfulness,
        answer_relevancy,
        context_precision,
        context_recall,
    ]

    # Only include answer_correctness if ground_truth is present in at least one case.
    has_gt = any(gt for gt in dataset["ground_truth"] if gt)
    if has_gt:
        if answer_correctness is not None:
            metrics.append(answer_correctness)
        metrics.append(answer_similarity)

    result = evaluate(
        dataset=dataset,
        metrics=metrics,
        llm=llm,
    )

    return {k: round(v, 4) if isinstance(v, float) else v for k, v in result.items()}


def main():
    args = parse_args()

    judge_model = os.environ.get("RAGAS_JUDGE_MODEL", "qwen3.5:9b")
    judge_url = os.environ.get("RAGAS_JUDGE_URL", "http://localhost:11434/v1")

    print("RAGAS Evaluation")
    print(f"  API:   {args.api_url}")
    print(f"  Cases: {args.cases}")
    print(f"  K:     {args.k}")
    print(f"  Judge: {judge_model} @ {judge_url}")
    print()

    cases = load_cases(args.cases)
    if not cases:
        print("No cases found.", file=sys.stderr)
        sys.exit(1)

    print(f"Loaded {len(cases)} cases. Querying PAA API...")

    llm = ChatOpenAI(
        model=judge_model,
        openai_api_base=judge_url,
        openai_api_key="unused",
        temperature=0,
    )

    dataset, case_details = build_ragas_dataset(cases, args.api_url, args.k)

    if len(dataset) == 0:
        print("No cases evaluated (all API calls failed).", file=sys.stderr)
        sys.exit(1)

    print(f"\nEvaluating {len(dataset)} cases with RAGAS...")
    scores = run_evaluation(dataset, llm)

    report = {
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "judge_model": judge_model,
        "judge_url": judge_url,
        "api_url": args.api_url,
        "cases_file": args.cases,
        "top_k": args.k,
        "summary": {
            "total_cases": len(cases),
            "evaluated_cases": len(dataset),
            **scores,
        },
        "cases": case_details,
    }

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(report, indent=2, ensure_ascii=False), encoding="utf-8")

    print(f"\nReport saved to: {args.out}")
    print("\nSummary:")
    for k, v in report["summary"].items():
        print(f"  {k}: {v}")


if __name__ == "__main__":
    main()
