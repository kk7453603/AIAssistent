# RAG Evaluation Pipeline (RAGAS)

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 2.2: Качество и память
**Статус:** approved

## Проблема

Текущий eval framework (bash + curl + jq) измеряет только retrieval quality (precision@k, recall@k, MRR). Нет оценки качества ответов — faithfulness, relevancy, correctness. Невозможно измерить, галлюцинирует ли модель или отвечает не по делу.

## Решение

Python-скрипт `ragas_eval.py` с RAGAS библиотекой. Вызывает PAA API, получает ответ + sources, оценивает через RAGAS с Ollama как LLM judge. Расширяет существующий формат кейсов опциональным `ground_truth`.

## Flow

```
cases.jsonl → ragas_eval.py → PAA API (/v1/rag/query) → answer + sources
                             → RAGAS metrics
                             → JSON report + stdout summary
```

## Формат кейсов (расширенный, обратно-совместимый)

```jsonl
{"id":"Q1","question":"Какой риск у документа X?","expected_filenames":["doc.txt"],"ground_truth":"Уровень риска — высокий"}
{"id":"Q2","question":"Что такое RAG?","expected_filenames":["rag.md"]}
```

`ground_truth` опционален. Если отсутствует — `answer_correctness` и `answer_similarity` пропускаются для этого кейса. Существующие кейсы работают без изменений.

## Метрики

| Метрика | Тип | Что оценивает |
|---------|-----|--------------|
| context_precision | retrieval | Доля релевантных чанков в top-K |
| context_recall | retrieval | Покрытие нужной информации в чанках |
| faithfulness | answer | Ответ не содержит фактов, отсутствующих в контексте |
| answer_relevancy | answer | Ответ по делу вопроса |
| answer_correctness | answer | Совпадение с golden answer (только если `ground_truth` задан) |
| answer_similarity | answer | Семантическая близость к golden answer (только если `ground_truth` задан) |

## LLM Judge

Ollama через OpenAI-compatible API (`http://localhost:11434/v1`).

Конфигурация через env:
- `RAGAS_JUDGE_MODEL` — модель для оценки (default: `qwen3.5:9b`)
- `RAGAS_JUDGE_URL` — URL Ollama API (default: `http://localhost:11434/v1`)

RAGAS использует `langchain_openai.ChatOpenAI` с `openai_api_base` для Ollama-compatible endpoint.

## Файловая структура

```
scripts/eval/
├── run.sh                    # существующий (precision@k, recall@k, MRR)
├── ragas_eval.py             # новый — RAGAS evaluation
├── ragas_requirements.txt    # Python зависимости (ragas, langchain-openai, requests)
├── cases.example.jsonl       # расширен ground_truth (опционально)
└── ...
```

## Makefile

```makefile
eval-ragas:
	python3 scripts/eval/ragas_eval.py \
		--api-url "$(EVAL_API_URL)" \
		--cases "$(EVAL_CASES)" \
		--k $(EVAL_K) \
		--out ./tmp/eval/ragas_report.json
```

## Выходной JSON report

```json
{
  "generated_at": "2026-03-28T18:00:00Z",
  "judge_model": "qwen3.5:9b",
  "summary": {
    "total_cases": 30,
    "evaluated_cases": 28,
    "context_precision": 0.72,
    "context_recall": 0.68,
    "faithfulness": 0.85,
    "answer_relevancy": 0.79,
    "answer_correctness": 0.65,
    "answer_similarity": 0.71
  },
  "cases": [...]
}
```

## Что НЕ входит

- Grafana dashboard для RAGAS метрик
- CI/CD интеграция (ручной запуск через `make eval-ragas`)
- Custom метрики помимо стандартных RAGAS
- Embeddings evaluation (используется RAGAS built-in)
