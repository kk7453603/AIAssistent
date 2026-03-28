# Adaptive Model Routing

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 2.3: Качество и память
**Статус:** approved

## Проблема

Все запросы идут к одной модели независимо от сложности. Простые вопросы ("привет", "какая погода") расходуют ресурсы мощной модели, а сложные задачи (code generation, multi-step reasoning) могут не получить достаточно мощную модель.

## Решение

Adaptive routing: определяем тип + сложность запроса, выбираем оптимальную модель из доступных. Гибридный classifier (rule-based fast path + LLM fallback). Автообнаружение моделей из Ollama.

## Complexity Tiers

Три уровня:
- `simple` — фактовые вопросы, приветствия, короткие ответы
- `complex` — multi-step reasoning, сравнения, анализ, объяснения
- `code` — генерация/анализ кода

### Конфигурация (explicit)

```env
MODEL_ROUTING={"simple":"llama3.1:8b","complex":"qwen3.5:9b","code":"qwen-coder:7b"}
```

Если задан — приоритет над auto-discovery.

## Complexity Classifier (гибрид)

### Fast path (rule-based)

Проверяет за ~0ms. Если уверенность высокая — решение сразу:

- Intent `code` (из существующего intent classifier) → tier `code`
- Длина < 30 символов + нет вопросительных слов → `simple`
- Ключевые слова сложности ("сравни", "объясни разницу", "проанализируй", "напиши план", "compare", "analyze", "explain why") → `complex`
- Если ни одно правило не сработало → `uncertain`

### LLM fallback

Если rule-based вернул `uncertain` — быстрый вызов к модели tier `simple` с промптом:
```
Classify the complexity of this user request. Answer with exactly one word: simple or complex.

Request: {question}
```

~200-500ms latency, но только для неоднозначных случаев.

## Auto-discovery моделей

### ModelDiscovery порт

```go
type ModelDiscovery interface {
    ListModels(ctx context.Context) ([]ModelInfo, error)
}

type ModelInfo struct {
    Name      string
    SizeBytes int64
}
```

### Реализация

`GET /api/tags` к Ollama → список установленных моделей.

### Auto-assign эвристики

При отсутствии `MODEL_ROUTING`:

```
Имя содержит "coder"/"code"  → tier "code"
Размер файла >= 8GB           → tier "complex" (примерно >= 13B параметров)
Размер файла < 8GB            → tier "simple"
```

Если моделей меньше чем tier'ов — все tier'ы указывают на дефолтную модель (`OLLAMA_GEN_MODEL`).

### Fallback

Ollama недоступен или нет моделей → все tier'ы → `OLLAMA_GEN_MODEL`.

## Интеграция

`ComplexityRouter` встраивается в `AgentChatUseCase` перед вызовом LLM:

```
user request → intent classifier (existing)
             → complexity classifier (new)
             → select model by tier
             → set context via routing.WithProvider
             → existing LLM call
```

Существующий `routing.Generator` + `WithProvider` уже умеет роутить по модели — нужно только выбрать правильную.

## Файловая структура

```
internal/core/domain/routing.go                    # ModelInfo, ComplexityTier, ModelRouting types
internal/core/usecase/complexity.go                # ComplexityClassifier (rule-based + LLM fallback)
internal/core/usecase/complexity_test.go
internal/infrastructure/llm/ollama/discovery.go    # ModelDiscovery implementation
internal/infrastructure/llm/ollama/discovery_test.go
internal/config/config.go                          # MODEL_ROUTING field + parser
internal/bootstrap/bootstrap.go                    # auto-discovery wiring
```

## Bootstrap flow

```
1. ollama.ListModels() → []ModelInfo
2. Если MODEL_ROUTING задан → parse JSON → explicit mapping
3. Если не задан → autoAssignTiers(models) → map[string]string
4. ComplexityRouter = NewComplexityRouter(mapping, generator)
5. Передать в AgentChatUseCase
```

## Что НЕ входит

- A/B тестирование моделей
- Fallback между моделями при ошибках (уже есть `fallback.Generator`)
- Динамическое переназначение tier'ов в runtime (только при старте)
- Routing для non-agent запросов (только agent chat)
