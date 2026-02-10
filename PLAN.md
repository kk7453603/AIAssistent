# PLAN: Personal AI Assistant (MVP)

## Цель MVP
Собрать сервис на Go, который:
- принимает документы;
- автоматически классифицирует их;
- индексирует чанки в Qdrant;
- использует индекс для базового RAG-ответа.

## Принципы
- Clean Architecture: разделение на `core` (domain/use cases/ports) и `adapters/infrastructure`.
- SOLID:
  - SRP: каждый слой и компонент отвечает за одну задачу;
  - OCP: расширение через интерфейсы портов;
  - LSP/ISP: узкие интерфейсы для классификатора, эмбеддера, репозитория и т.д.;
  - DIP: use case зависит от абстракций, не от конкретных клиентов.

## Этапы

### 1. Планирование и структура
- [x] Создать `PLAN.md`.
- [x] Зафиксировать структуру каталогов проекта.
- [x] Подготовить `.env.example` с конфигами.

### 2. Инфраструктура разработки (Docker Compose)
- [x] Подготовить сервисы: `ollama`, `qdrant`, `postgres`, `minio`, `nats`.
- [x] Добавить `api` и `worker` контейнеры.
- [x] Настроить volume для общего доступа к загруженным файлам.

### 3. Каркас Go-проекта (Clean Architecture)
- [x] Инициализировать `go.mod`.
- [ ] Создать слои:
  - [x] `internal/core/domain`
  - [x] `internal/core/ports`
  - [x] `internal/core/usecase`
  - [x] `internal/adapters/http`
  - [x] `internal/infrastructure/*`
  - [x] `internal/config`
- [x] Создать `cmd/api` и `cmd/worker`.

### 4. MVP-функциональность
- [x] Upload API: прием файла и сохранение метаданных.
- [x] Асинхронная обработка через NATS.
- [ ] Worker pipeline:
  - [x] извлечение текста;
  - [x] классификация через Ollama (structured JSON);
  - [x] чанкинг;
  - [x] эмбеддинги через Ollama;
  - [x] upsert в Qdrant;
  - [x] обновление статуса в Postgres.
- [x] Query API: retrieval из Qdrant + генерация ответа.

### 5. Проверка и документация
- [x] Добавить SQL-миграцию `documents`.
- [x] Проверить `go test ./...` и сборку.
- [x] Обновить `PLAN.md` статусами выполнения.
- [x] Добавить краткий `README.md` с запуском MVP.

## Критерии готовности MVP
- Документ успешно проходит путь `uploaded -> processing -> ready` (или `failed`).
- Для `ready` документа существуют вектора в Qdrant и метаданные в Postgres.
- Запрос к RAG endpoint возвращает ответ с учетом найденных чанков.
