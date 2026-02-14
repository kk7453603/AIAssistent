# ADR-0001: Clean Architecture Refactor (SOLID + KISS)

## Status
Accepted (2026-02-14)

## Context
Проект имел базовое разделение на `core/adapters/infrastructure`, но:
- HTTP-слой зависел от concrete use case типов;
- OpenAI-compatible адаптер содержал несколько ответственностей в одном файле;
- Use case обработки документа совмещал оркестрацию, статус-политику и побочные эффекты;
- доменные ошибки не были типизированы для единообразного transport mapping.

## Decision
1. Разделить порты на inbound/outbound контракты.
2. Сделать HTTP-адаптер зависимым от inbound интерфейсов.
3. Добавить типизированные доменные ошибки и единый HTTP error mapping.
4. Декомпозировать `ProcessDocumentUseCase` на этапы pipeline.
5. Декомпозировать OpenAI-compatible адаптер по зонам ответственности.

## Consequences
### Positive
- Снижение связности между transport и application слоями.
- Проще тестировать use case и adapter независимо.
- Предсказуемое преобразование доменных ошибок в HTTP-коды.
- Легче расширять OpenAI-compatible поведение без роста god-file.

### Trade-offs
- Больше файлов и интерфейсов.
- Нужна дисциплина поддержки inbound/outbound контрактов.

## Defaults and constraints
- Внешние API-контракты сохраняются backward-compatible.
- Технологический стек не меняется: PostgreSQL, NATS, Ollama, Qdrant.
- Приоритет: тестируемость и поддерживаемость выше микрооптимизаций.
