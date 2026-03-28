# C4 Architecture — Personal AI Assistant

Архитектурная документация проекта в нотации [C4 Model](https://c4model.com/) (Simon Brown).

Все диаграммы в формате Mermaid — рендерятся на GitHub.

## Уровни

### Level 1 — System Context
- [System Context](c4-context.md) — PAA и внешний мир

### Level 2 — Containers
- [Containers](c4-containers.md) — API, Worker, UI, инфраструктура

### Level 3 — Components (по фичам)
- [RAG Pipeline](c4-rag-pipeline.md) — загрузка, обработка, поиск документов
- [Agent Loop](c4-agent-loop.md) — каскадный поиск, инструменты, память
- [Obsidian](c4-obsidian.md) — синхронизация vault, создание заметок
- [Knowledge Graph](c4-knowledge-graph.md) — Neo4j, связи, расширение запросов
- [Multi-Agent](c4-multi-agent.md) — оркестратор специалистов
- [Self-Improvement](c4-self-improvement.md) — события, обратная связь, улучшения
- [Scheduled Tasks](c4-scheduled-tasks.md) — cron-планировщик
- [MCP](c4-mcp.md) — MCP-сервер и внешние MCP-клиенты
- [Tauri UI](c4-tauri-ui.md) — десктопное приложение
- [Monitoring](c4-monitoring.md) — Prometheus, Grafana, алерты

## Глоссарий

| Термин | Значение |
|--------|----------|
| PAA | Personal AI Assistant — основная система |
| RAG | Retrieval-Augmented Generation — генерация с поиском |
| MCP | Model Context Protocol — протокол контекста моделей |
| SSE | Server-Sent Events — потоковая передача |
| Chunk | Фрагмент документа для векторного поиска |
| Embedding | Векторное представление текста |
| Reranking | Переранжирование результатов поиска |
