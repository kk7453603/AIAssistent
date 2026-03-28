# RAG Evaluation Pipeline (RAGAS) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Python-based RAGAS evaluation pipeline that measures faithfulness, relevancy, correctness, and context quality of RAG answers.

**Architecture:** Single Python script calls PAA API, collects answers + sources, evaluates via RAGAS library with Ollama as LLM judge. Integrates with existing Makefile and case format.

**Tech Stack:** Python 3, ragas, langchain-openai, requests.

**Spec:** `docs/superpowers/specs/2026-03-28-ragas-evaluation.md`

---

### Task 1: Create Python requirements file

**Files:**
- Create: `scripts/eval/ragas_requirements.txt`

- [ ] **Step 1: Create requirements file**

Create `scripts/eval/ragas_requirements.txt`:

```
ragas>=0.2.0
langchain-openai>=0.3.0
datasets>=2.14.0
requests>=2.31.0
```

- [ ] **Step 2: Commit**

```bash
git add scripts/eval/ragas_requirements.txt
git commit -m "feat(eval): add RAGAS Python requirements"
```

---

### Task 2: Implement ragas_eval.py

**Files:**
- Create: `scripts/eval/ragas_eval.py`

- [ ] **Step 1: Create the evaluation script**

Create `scripts/eval/ragas_eval.py`:

```python
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

    for case in cases:
        question = case["question"]
        case_id = case.get("id", "unknown")
        gt = case.get("ground_truth", "")

        try:
            result = query_paa(api_url, question, k)
        except Exception as e:
            print(f"  SKIP {case_id}: API error — {e}", file=sys.stderr)
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

    if answer_correctness is not None:
        # Only include answer_correctness if ground_truth is present in at least one case
        has_gt = any(gt for gt in dataset["ground_truth"] if gt)
        if has_gt:
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

    print(f"RAGAS Evaluation")
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

    print(f"Evaluating {len(dataset)} cases with RAGAS...")
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
    print(f"\nSummary:")
    for k, v in report["summary"].items():
        print(f"  {k}: {v}")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Make executable**

Run: `chmod +x scripts/eval/ragas_eval.py`

- [ ] **Step 3: Commit**

```bash
git add scripts/eval/ragas_eval.py
git commit -m "feat(eval): RAGAS evaluation script with faithfulness, relevancy, correctness"
```

---

### Task 3: Update Makefile and cases

**Files:**
- Modify: `Makefile`
- Modify: `scripts/eval/cases.example.jsonl`

- [ ] **Step 1: Add eval-ragas target to Makefile**

Add to `.PHONY` line: `eval-ragas`

Add after the `eval-compare` target:

```makefile
eval-ragas:
	python3 scripts/eval/ragas_eval.py \
		--api-url "$(EVAL_API_URL)" \
		--cases "$(EVAL_CASES)" \
		--k $(EVAL_K) \
		--out ./tmp/eval/ragas_report.json
```

- [ ] **Step 2: Update example cases with ground_truth**

Replace `scripts/eval/cases.example.jsonl` with:

```jsonl
# Replace expected_filenames with files that are actually indexed in your Qdrant collection.
{"id":"EX001","question":"What is the risk level for document doc_0001_support_ap-south.txt?","expected_filenames":["doc_0001_support_ap-south.txt"],"ground_truth":"The risk level is documented in doc_0001_support_ap-south.txt."}
{"id":"EX002","question":"What is the gross margin for document doc_0002_support_us-west.txt in 2024-Q1?","expected_filenames":["doc_0002_support_us-west.txt"],"ground_truth":"The gross margin information is in doc_0002_support_us-west.txt for 2024-Q1."}
{"id":"EX003","question":"What is the incident type for document doc_0003_risk_us-west.txt?","expected_filenames":["doc_0003_risk_us-west.txt"]}
```

Note: EX003 intentionally has no `ground_truth` to demonstrate optional field.

- [ ] **Step 3: Commit**

```bash
git add Makefile scripts/eval/cases.example.jsonl
git commit -m "feat(eval): add eval-ragas Makefile target and update example cases with ground_truth"
```

---

### Task 4: Verify end-to-end

- [ ] **Step 1: Install Python dependencies**

Run: `pip install -r scripts/eval/ragas_requirements.txt`

- [ ] **Step 2: Test script loads without errors**

Run: `python3 scripts/eval/ragas_eval.py --help`
Expected: Shows usage/help text without import errors.

- [ ] **Step 3: Test with dry run (if PAA is not running)**

Run: `python3 -c "from ragas.metrics import faithfulness, answer_relevancy, context_precision, context_recall; print('RAGAS imports OK')"`
Expected: `RAGAS imports OK`

- [ ] **Step 4: Push**

```bash
git push origin main
```
